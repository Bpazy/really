package opendota

import (
	"fmt"
	"github.com/Bpazy/behappy/bjson"
	"github.com/Bpazy/behappy/http"
	"github.com/Bpazy/behappy/models"
	"github.com/sirupsen/logrus"
)

func GetMatchPlayers(playerID string, limit int) []models.MatchPlayer {
	r, err := http.Client.R().Get(fmt.Sprintf("https://api.opendota.com/api/players/%s/matches?limit=%d", playerID, limit))
	if err != nil {
		logrus.Printf("从 opendota 获取玩家比赛列表失败: %+v", err)
		return []models.MatchPlayer{}
	}

	var mps []models.MatchPlayer
	bjson.MustJsonUnmarshal(r.Body(), &mps)
	return mps
}
