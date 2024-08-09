package middleware

import (
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"hamsterbot/internal/app/models"
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
		/*if c.Sender().ID == 5953539293 && strings.Contains(c.Message().Text, "/") {
			return c.Send("Вы заблокированы до выяснения обстоятельств.")
		}*/

		data, err := e.User.GetUserById(c.Sender().ID)
		if err != nil {
			err := e.User.AddUser(c.Sender().ID, c.Sender().Username)
			if err != nil {
				logger.Error("ошибка добавления юзера", zap.Error(err))
				return err
			}

			return next(c)
		}

		if data["mute"].(models.Mute) != (models.Mute{}) {
			err := e.Bot.Delete(c.Message())
			if err != nil {
				return err
			}
		}

		args := c.Args()

		if strings.Contains(strings.Join(args, " "), "hamsteryep_bot") ||
			(c.Message().ReplyTo != nil && c.Message().ReplyTo.Sender != nil && c.Message().ReplyTo.Sender.Username == "hamsteryep_bot" && strings.Contains(c.Message().Text, "/")) {
			return c.Send("Ошибка: нельзя проводить какие-либо операции над ботом.")
		}

		return next(c)
	}
}
