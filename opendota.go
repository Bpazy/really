package really

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
)

func SubscribeFunc() {
	var sps []*SubscribePlayer
	if err := db.Find(&sps).Error; err != nil {
		log.Println("没有订阅的玩家")
	}

	for _, sp := range sps {
		playerDetailRes, err := client.R().Get(fmt.Sprintf("https://api.opendota.com/api/players/%s/matches?limit=1", sp.PlayerId))
		if err != nil {
			log.Printf("从 opendota 获取玩家比赛列表失败: %+v\n", err)
		}

		var matchPlayers []*MatchPlayer
		JsonUnmarshal(playerDetailRes.Body(), &matchPlayers)

		for _, mp := range matchPlayers {
			mp.PlayerID = sp.PlayerId
			s := map[string]interface{}{
				"match_id":  mp.MatchID,
				"player_id": sp.PlayerId,
			}
			if err := db.Where(s).First(&MatchPlayer{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				// 新比赛
				log.Printf("探测到新的比赛：%d\n", mp.MatchID)
				pretty := fmt.Sprintf("英雄: %s\n等级: %s\n\n击杀: %d, 死亡: %d, 助攻: %d", mp.HeroName(), mp.SkillString(), mp.Kills, mp.Deaths, mp.Assists)
				SendGroupMessage(sp.GroupId, fmt.Sprintf("「%s」有新「%s」的比赛了: \n\n%s", sp.Name(), mp.MatchResultString(), pretty))
				db.Create(mp)
			}
		}
	}
}

func InitHeros() {
	b := Get("https://api.opendota.com/api/heroes")

	var heros []*Hero
	JsonUnmarshal(b, &heros)

	for _, hero := range heros {
		db.Create(&hero)
	}
}