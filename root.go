package really

import (
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"gopkg.in/resty.v1"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// buildVer represents 'really' build version
	buildVer string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "really",
		Short: "TODO",
		Long: `TODO
`,
		Run: func(cmd *cobra.Command, args []string) {
			Run()
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "版本号",
		Long:  `查看 really 的版本号`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Println(buildVer)
		},
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func Run() {
	db := initDB()

	client := loginDotaMax()

	startCron(client, db)

	serveMirai()
}

func initDB() *sql.DB {
	userHomeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(userHomeDir, ".really.db")
	_, dbNotExistsErr := os.Open(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	if dbNotExistsErr != nil {
		sqlStmt := `
			CREATE TABLE "match_player" (
			  "id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
			  "match_id" text NOT NULL,
			  "player_id" text NOT NULL,
			  "hero" text NOT NULL,
			  "match_mode" text NOT NULL,
			  "match_result" TEXT NOT NULL,
			  "match_kda" TEXT NOT NULL,
			  "match_level" TEXT NOT NULL,
			  "create_time" TEXT NOT NULL,
			  "modify_time" TEXT NOT NULL
			);
		`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			panic(err)
		}
	}
	return db
}

var r = regexp.MustCompile("\\d+")

func startCron(client *resty.Client, db *sql.DB) {
	c := cron.New()
	c.AddFunc("@every 1s", func() {
		playerDetailRes, err := client.R().Get("http://dotamax.com/player/detail/122155653/")
		if err != nil {
			log.Printf("获取用户详情失败: %+v, 尝试重新登录\n", err)
			client = loginDotaMax()
			return
		}
		dom, err := goquery.NewDocumentFromReader(strings.NewReader(playerDetailRes.String()))
		if err != nil {
			log.Printf("解析用户详情 DOM 失败: %+v\n", err)
			return
		}

		playerDetails := dom.Find(".table-player-detail")
		// Get(0): 常用英雄
		// Get(1): 最近比赛
		// Get(2): 最高记录
		s := dom.FindNodes(playerDetails.Get(1))
		s.Find("tr").Each(func(i int, cs *goquery.Selection) {
			// 每一场比赛
			var lines []string
			cs.Find("td").Each(func(i int, cs2 *goquery.Selection) {
				lines = append(lines, strings.TrimSpace(cs2.Text()))
			})
			hero := lines[0]
			matchId := r.FindString(lines[1])
			matchMode := strings.SplitAfter(lines[1], matchId)[1]
			result := lines[3]
			kda := lines[4]
			level := lines[5]
			log.Printf("英雄: %s, 比赛ID: %s, 比赛模式: %s, 结果: %s, KDA: %s, 等级: %s\n", hero, matchId, matchMode, result, kda, level)

			sqlStmt := `INSERT INTO "match_player" ("match_id", "player_id", "hero", "match_mode", "match_result", "match_kda", "match_level", "create_time", "modify_time" ) 
						VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
			_, err := db.Exec(sqlStmt, matchId, "122155653", hero, matchMode, result, kda, level, time.Now(), time.Now())
			if err != nil {
				panic(err)
			}
		})
	})

	c.Start()
}

func loginDotaMax() *resty.Client {
	config := InitConfig()
	client, u := initRestyClient(config)

	// 如果有 Cookie 则跳过登录
	if len(client.GetClient().Jar.Cookies(u)) != 0 {
		log.Printf("使用 Cookie 登录\n")
		return client
	}

	getLoginPageRes, err := client.R().Get("http://dotamax.com/login/")
	if err != nil {
		panic(err)
	}

	data := map[string]string{
		"csrfmiddlewaretoken": getCsrfToken(getLoginPageRes.String()),
		"phoneNumCipherb64":   encrypt(""),
		"usernameCipherb64":   encrypt(config.DotaMax.Username),
		"passwordCipherb64":   encrypt(config.DotaMax.Password),
		"account-type":        "2",
		"src":                 "None",
	}
	loginRes, err := client.R().
		SetFormData(data).
		Post("http://dotamax.com/accounts/login/")
	if err != nil {
		panic(err)
	}

	body := loginRes.String()
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	loginReply := dom.Find(".login-reply").Text()
	if loginReply != "" {
		log.Fatalf("登录 DotaMax 失败: %s\n", loginReply)
	}
	if strings.Contains(body, "随机征召") {
		log.Printf("登录 DotaMax 成功\n")
	}

	config.SetCookies(loginRes.Cookies())
	return client
}

func getCsrfToken(body string) string {
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	csrfToken, exists := dom.Find("[name=csrfmiddlewaretoken]").Attr("value")
	if !exists {
		panic("未找到 csrfmiddlewaretoken")
	}
	return csrfToken
}

func initRestyClient(c *configuration) (*resty.Client, *url.URL) {
	client := resty.New()

	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	parseUrl, err := url.Parse("http://dotamax.com/")
	if err != nil {
		panic(err)
	}

	var cookies []*http.Cookie
	if c.DotaMax.Cookies != "" {
		err := json.Unmarshal([]byte(c.DotaMax.Cookies), &cookies)
		if err != nil {
			log.Fatalf("Cookie 格式错误: %+v", err)
		}
		jar.SetCookies(parseUrl, cookies)
	}
	client.SetCookieJar(jar)

	return client, parseUrl
}

func encrypt(content string) string {
	rsaE := "10001"
	rsaN := "B81E72A33686A201B0AC009D679750990E3D168670DC6F9452C24E5A4C299AC002C6C89C3CB38784AEA95D66B7B3E9CA950EB9EEFB4EF61383EDDB67C35727F9CA87EE3238346C66D042B35812179501F472AD4F3BA19E701256FE0435AB856E5C5BEA24A2387153023CD4CD43CDA7260FCC1E2E49C14102C253F559F9A45D59DF5004A017B1239448A9A001D276CAD12535DEDE89FFBD57D75BBC9B575530DDD1B7FAD46064AD3C640CBD017F58981215B2EE17CBE175C36570C5235902818648577234E70E81133B088164F98E605D0D6E69A6095A32A72511E9AC901727B635CE2E8002A7B0EC8D012606903BCB825E60C7B6619FFCED4401E693F5EC68AB"

	n := new(big.Int)
	n, ok := n.SetString(rsaN, 16)
	if !ok {
		panic("public key should be hexadecimal")
	}

	hexRsaE, err := strconv.ParseInt(rsaE, 16, 64)
	if err != nil {
		panic(err)
	}
	encryptedData, err := rsa.EncryptPKCS1v15(rand.Reader, &rsa.PublicKey{
		N: n,
		E: int(hexRsaE),
	}, []byte(content))
	if err != nil {
		panic(err)
	}
	return linebrk(base64.StdEncoding.EncodeToString(encryptedData), 64)
}

func linebrk(s string, n int) string {
	var ret = ""
	var i = 0
	for i+n < len(s) {
		ret += s[i:i+n] + "\n"
		i += n
	}
	return ret + s[i:len(s)]
}

// serveMirai 开启 Mirai事件上报监听器
func serveMirai() {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/post", func(c *gin.Context) {
		all, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}
		log.Printf("接受到来自 Mirai 的上报: %s\n", string(all))
		c.JSON(200, nil)
	})
	log.Fatal(r.Run("0.0.0.0:10000"))
}

type FriendMessage struct {
	SessionKey   string         `json:"sessionKey"`
	Target       int            `json:"target"`
	MessageChain []PlainMessage `json:"messageChain"`
}

type PlainMessage struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func NewPlainMessage(text string) PlainMessage {
	return PlainMessage{
		Type: "Plain",
		Text: text,
	}
}

func NewFriendMessage(target int, text string) *FriendMessage {
	return &FriendMessage{
		SessionKey: "",
		Target:     target,
		MessageChain: []PlainMessage{
			NewPlainMessage(text),
		},
	}
}
