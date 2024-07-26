package payments

import (
	"fmt"
	"gopkg.in/telebot.v3"
	"hamsterbot/pkg/logger"
	"strconv"
	"strings"
)

type Payment interface {
	Pay(from string, to string, amount int) (int, error)
}

type Endpoint struct {
	Payment Payment
}

func (e *Endpoint) PayHandler(c telebot.Context) error {
	logger.Debug("Вызван обработчик Pay")
	var username string
	var amount int

	args := c.Args()

	// /pay <username> <сумма>
	if len(args) == 2 {
		username = args[0]
		amountStr := args[1]

		var err error
		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /pay username 100")
		}
		// /pay <сумма>
	} else if len(args) == 1 && c.Message().ReplyTo != nil {
		username = c.Message().ReplyTo.Sender.Username

		amountStr := args[0]

		var err error
		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /pay username 100")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /pay username сумма или ответьте командой /pay сумма на сообщение.")
	}

	if amount < 0 {
		return c.Send("Ошибка: число не может быть отрицательным.")
	}

	balance, err := e.Payment.Pay(c.Sender().Username, username, amount)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d", err.Error(), balance))
	}

	return c.Send(fmt.Sprintf("Платеж пользователю @%s на сумму %d был успешно обработан. Ваш текущий баланс: %d", strings.Trim(username, "@"), amount, balance))
}
