package middleware

import (
	"fmt"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"strings"
)

type User interface {
	GetUserById(id int64) (map[string]interface{}, error)
	AddUser(id int64, username string) error
}

type Endpoint struct {
	Bot  *tele.Bot
	User User
}

func (e *Endpoint) IsUser(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		data, err := e.User.GetUserById(c.Sender().ID)
		if err != nil {
			return err
		}

		if len(data) == 0 {
			err := e.User.AddUser(c.Sender().ID, c.Sender().Username)
			if err != nil {
				return err
			}

			return next(c)
		}

		cacheKey := fmt.Sprintf("mute:%d", c.Sender().ID)

		exists, err := cache.Rdb.Exists(cache.Ctx, cacheKey).Result()
		if err != nil {
			logger.Warn("Ошибка проверки наличия ключа в кеше", zap.Error(err))
		}
		if exists != 0 {
			err := e.Bot.Delete(c.Message())
			if err != nil {
				return err
			}
		}

		args := c.Args()

		if strings.Contains(strings.Join(args, " "), "hamsteryep_bot") ||
			(c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil && c.Message().ReplyTo.Sender.Username == "hamsteryep_bot") {
			return c.Send("Ошибка: нельзя проводить какие-либо операции над ботом.")
		}

		return next(c)
	}
}
