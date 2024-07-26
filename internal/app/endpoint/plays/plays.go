package plays

import (
	"fmt"
	"gopkg.in/telebot.v3"
	"hamsterbot/pkg/logger"
	"strconv"
	"strings"
)

type Play interface {
	Slots(username string, amount int) (bool, []string, int, int, error)
	Steal(to string, from string, amount int) (bool, int, error)
}

type Endpoint struct {
	Play Play
}

func (e *Endpoint) SlotsHandler(c telebot.Context) error {
	logger.Debug("Вызван обработчик Slots")
	var oldAmount int

	args := c.Args()

	if len(args) == 1 { // /slots <amount>
		var err error
		oldAmount, err = strconv.Atoi(args[0])
		if err != nil {
			return err
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /slots <amount>.")
	}

	if oldAmount < 0 {
		return c.Send("Ошибка: число не может быть отрицательным.")
	}

	win, result, amount, balance, err := e.Play.Slots(c.Sender().Username, oldAmount)
	if err != nil {
		if err.Error() == "недостаточно средств" {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\n%s | %s | %s\n\n", oldAmount, result[0], result[1], result[2])
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", amount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	return c.Send(resultMsg)
}

func (e *Endpoint) StealHandler(c telebot.Context) error {
	logger.Debug("Вызван обработчик Steals")
	var username string
	var amount int

	args := c.Args()

	if len(args) == 2 {
		username = args[0]
		amountStr := args[1]

		var err error
		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /steal username 100")
		}
		// /pay <сумма>
	} else if len(args) == 1 && c.Message().ReplyTo != nil {
		username = c.Message().ReplyTo.Sender.Username

		amountStr := args[0]

		var err error
		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /steal username 100")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /steal username сумма или ответьте командой /steal сумма на сообщение.")
	}

	if amount < 0 {
		return c.Send("Ошибка: число не может быть отрицательным.")
	}

	win, balance, err := e.Play.Steal(c.Sender().Username, username, amount)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Попытка украсть %d зеток у @%s: ", amount, strings.Trim(username, "@"))
	if win {
		resultMsg += fmt.Sprintf("✅ Успешно! \n\n Ваш баланс: %d зеток\n", balance)
	} else {
		resultMsg += fmt.Sprintf("🚫 Неудача( \n\n Ваш баланс: %d зеток\n", balance)
	}

	return c.Send(resultMsg)
}
