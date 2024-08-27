package users

import (
	"fmt"
	"gopkg.in/telebot.v3"
	"hamsterbot/internal/app/models"
	"strings"
	"time"
)

type User interface {
	GetUserByUsername(username string) (map[string]interface{}, error)
	AddUser(id int64, username string) error
	GetTopByBalance() ([]models.UserTop, error)
	GetTopByLVL() ([]models.UserTop, error)
	GetTopByIncome() ([]models.UserTop, error)
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
		return c.Send("Ошибка: " + err.Error())
	}

	messageSend := fmt.Sprintf("📌 Информация о пользователе @%s:\n\n👉 LVL: %d ур.\n👉 Баланс: %d зеток\n👉 Доход: %d зеток/ч", strings.Trim(username, "@"), data["lvl"].(int64), data["balance"].(int64), data["income"].(int64))
	if data["mute"].(models.Mute) != (models.Mute{}) {
		jsonStartMute, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", data["mute"].(models.Mute).StartMute)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}
		jsonDuration := time.Duration(data["mute"].(models.Mute).Duration)

		location := time.FixedZone("UTC+3", 3*60*60)
		endTime := jsonStartMute.Add(jsonDuration).In(location).Format("2006-01-02 15:04:05")

		messageSend += fmt.Sprintf("\n👉 Блокировка будет снята %s", endTime)
	}
	if data["selfmute"].(models.Mute) != (models.Mute{}) {
		jsonStartMute, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", data["selfmute"].(models.Mute).StartMute)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}
		jsonDuration := time.Duration(data["selfmute"].(models.Mute).Duration)

		location := time.FixedZone("UTC+3", 3*60*60)
		endTime := jsonStartMute.Add(jsonDuration).In(location).Format("2006-01-02 15:04:05")

		messageSend += fmt.Sprintf("\n👉 Самоблокировка будет снята %s", endTime)
	}

	return c.Send(messageSend)
}

func (e *Endpoint) TopHandler(c telebot.Context) error {
	var top string
	args := c.Args()

	if len(args) == 1 {
		top = args[0]
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /top balance/lvl/income или симлинки /topb, /topl, /topi соответственно.")
	}

	return e.TopHandlerCommand(c, top)
}

func (e *Endpoint) TopHandlerCommand(c telebot.Context, top string) error {
	var resultMsg string
	var data []models.UserTop
	var err error
	switch top {
	case "balance":
		data, err = e.User.GetTopByBalance()
		resultMsg = "🎰 Топ 10 игроков по балансу:\n\n"
	case "lvl":
		data, err = e.User.GetTopByLVL()
		resultMsg = "🎰 Топ 10 игроков по уровню:\n\n"
	case "income":
		data, err = e.User.GetTopByIncome()
		resultMsg = "🎰 Топ 10 игроков по доходу:\n\n"
	}
	if err != nil {
		return err
	}

	for _, topValue := range data {
		resultMsg += fmt.Sprintf("- %s: %d\n", topValue.Username, topValue.Value)
	}

	return c.Send(resultMsg)
}
