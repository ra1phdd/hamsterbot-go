package payments

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
	"hamsterbot/pkg/logger"
	"strconv"
	"strings"
)

type Payment interface {
	Pay(from string, to string, amount int) (int64, error)
	PayAdm(to string, amount int) (int64, error)
}

type User interface {
	GetUserById(id int64) (map[string]interface{}, error)
	GetUserByUsername(username string) (map[string]interface{}, error)
	GetBankBalance() (map[string]interface{}, error)
}

type Endpoint struct {
	Payment Payment
	User    User
}

func (e *Endpoint) PayHandler(c telebot.Context) error {
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

	if c.Sender().Username == strings.Trim(username, "@") {
		return c.Send("Ошибка: нельзя перевести деньги самому себе.")
	}

	if amount < 0 {
		return c.Send("Ошибка: число не может быть отрицательным.")
	}

	balance, err := e.Payment.Pay(c.Sender().Username, username, amount)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток", err.Error(), balance))
	}

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) отправил деньги пользователю @%s", c.Sender().Username, c.Sender().ID, strings.Trim(username, "@")),
		c.Chat().ID, c.Chat().Title, zap.Int("amount", amount), zap.Int64("balance", balance))
	return c.Send(fmt.Sprintf("Платеж пользователю @%s на сумму %d зеток был успешно обработан. Ваш текущий баланс: %d зеток", strings.Trim(username, "@"), amount, balance))
}

func (e *Endpoint) PayAdmHandler(c telebot.Context) error {
	if c.Sender().ID != 1230045591 {
		return nil
	}
	logger.Debug("Вызван обработчик PayAdm")
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

	balance, err := e.Payment.PayAdm(username, amount)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток", err.Error(), balance))
	}

	return c.Send(fmt.Sprintf("Платеж пользователю @%s на сумму %d зеток был успешно обработан", strings.Trim(username, "@"), amount))
}

func (e *Endpoint) BankHandler(c telebot.Context) error {
	args := c.Args()

	switch len(args) {
	case 0:
		err := e.GetBankData(c)
		if err != nil {
			return err
		}
	case 1:
		if args[0] == "info" {
			bank, err := e.User.GetUserByUsername(fmt.Sprintf("bank_%d_%s", c.Sender().ID, c.Sender().Username))
			if err != nil {
				return c.Send("Неизвестная ошибка")
			}

			return c.Send(fmt.Sprintf("📌 Информация о личном счёте @%s в банке:\n\n👉 Баланс: %d зеток\n", c.Sender().Username, bank["balance"].(int64)) + "👉 Ставка: 3% дневных")
		}
	case 2: // /bank pay <сумма>
		if args[0] == "pay" {
			amount, err := strconv.Atoi(args[1])
			if err != nil {
				return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /bank pay 100")
			}

			var userBalance, bankBalance int64
			user, err := e.User.GetUserById(c.Sender().ID)
			if err != nil {
				return c.Send("Неизвестная ошибка")
			}
			bank, err := e.User.GetUserByUsername(fmt.Sprintf("bank_%d_%s", c.Sender().ID, c.Sender().Username))
			if err != nil {
				return c.Send("Неизвестная ошибка")
			}
			userBalance = user["balance"].(int64)
			bankBalance = bank["balance"].(int64)

			if amount > 0 {
				userBalance, err = e.Payment.Pay(c.Sender().Username, fmt.Sprintf("bank_%d_%s", c.Sender().ID, c.Sender().Username), -amount)
				if err != nil {
					return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток", err.Error(), userBalance))
				}
				bankBalance -= int64(amount)
			} else if amount < 0 {
				bankBalance, err = e.Payment.Pay(fmt.Sprintf("bank_%d_%s", c.Sender().ID, c.Sender().Username), c.Sender().Username, amount)
				if err != nil {
					return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток", err.Error(), userBalance))
				}
				userBalance -= int64(amount)
			} else {
				return c.Send("Ошибка: число не может быть нулевым.")
			}

			logger.Infof(fmt.Sprintf("Пользователь @%s (%d) отправил деньги в личный банк", c.Sender().Username, c.Sender().ID),
				c.Chat().ID, c.Chat().Title, zap.Int("amount", amount), zap.Int64("userBalance", userBalance), zap.Int64("bankBalance", bankBalance))
			return c.Send(fmt.Sprintf("Перевод на личный счет в банке на сумму %d зеток был успешно обработан. Ваш текущий баланс: %d зеток. Баланс вашего счета в банке: %d зеток", amount, userBalance, bankBalance))

		}
	default:
		return c.Send("Неизвестная команда. Для помощи напишите /help")
	}

	return nil
}

func (e *Endpoint) GetBankData(c telebot.Context) error {
	data, err := e.User.GetBankBalance()
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	return c.Send(fmt.Sprintf("📌 Информация о банке:\n\n👉 Общий баланс: %d зеток\nИз них хранятся на счетах пользователей: %d зеток", data["bank"].(int64)+data["users"].(int64), data["users"].(int64)))
}
