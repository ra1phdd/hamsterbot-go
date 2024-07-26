package users

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"strings"
)

type User interface {
	GetUserByUsername(username string) (map[string]interface{}, error)
	AddUser(id int64, username string) error
}

type Endpoint struct {
	User User
}

func (e *Endpoint) GetUserData(c telebot.Context) error {
	var username string
	args := c.Args()

	// /user <username>
	if len(args) == 1 {
		username = args[0]
		// /user
	} else if len(args) == 0 {
		if c.Message().ReplyTo != nil {
			username = c.Message().ReplyTo.Sender.Username
		} else {
			username = c.Sender().Username
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /user username или ответьте командой /user на сообщение.")
	}

	data, err := e.User.GetUserByUsername(username)
	if err != nil {
		return err
	}

	var messageSend string
	cacheKey := fmt.Sprintf("mute:%d", data["id"].(int64))

	exists, err := cache.Rdb.Exists(cache.Ctx, cacheKey).Result()
	if err != nil {
		logger.Warn("Ошибка проверки наличия ключа в кеше", zap.Error(err))
	}
	if exists != 0 {
		time, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
		if err != nil {
			return err
		}

		messageSend = fmt.Sprintf("📌 Информация о пользователе @%s:\n👉 Баланс: %d\n👉 Блокировка будет снята %s (UTC)", strings.Trim(username, "@"), data["balance"].(int), time)
	} else {
		messageSend = fmt.Sprintf("📌 Информация о пользователе @%s:\n👉 Баланс: %d", strings.Trim(username, "@"), data["balance"].(int))
	}

	return c.Send(messageSend)
}
