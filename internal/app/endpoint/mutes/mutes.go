package mutes

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
	"hamsterbot/pkg/logger"
	"strings"
)

type Mute interface {
	Mute(from string, to string, durationStr string) (int, int, error)
	Unmute(from string, to string) (int, int, error)
}

type Endpoint struct {
	Mute Mute
}

func (e *Endpoint) MuteHandler(c telebot.Context) error {
	logger.Debug("Вызван обработчик Mute")
	var username, duration string
	args := c.Args()

	if len(args) == 2 { // /mute <username> <время>
		username = args[0]
		duration = args[1]
	} else if len(args) == 1 && c.Message().ReplyTo != nil { // /mute <время>
		username = c.Message().ReplyTo.Sender.Username
		duration = args[0]
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /mute <username> <время> или ответьте командой /mute <время> время на сообщение.")
	}

	logger.Debug("Получение аргументов", zap.String("username", username), zap.String("duration", duration))
	balance, amount, err := e.Mute.Mute(c.Sender().Username, username, duration)
	if err != nil {
		if err.Error() == "недостаточно средств" {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	return c.Send(fmt.Sprintf("Пользователь @%s замучен на %s за %d зеток. Ваш текущий баланс: %d зеток.", strings.Trim(username, "@"), duration, amount, balance))
}

func (e *Endpoint) UnmuteHandler(c telebot.Context) error {
	logger.Debug("Вызван обработчик Unmute")
	var username string
	args := c.Args()

	if len(args) == 1 { // /unmute <username>
		username = args[0]
	} else if len(args) == 0 { // /unmute
		if c.Message().ReplyTo != nil {
			username = c.Message().ReplyTo.Sender.Username
		} else {
			username = c.Sender().Username
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /unmute <username> или ответьте командой /unmute время на сообщение.")
	}

	logger.Debug("Получение аргументов", zap.String("username", username))
	balance, amount, err := e.Mute.Unmute(c.Sender().Username, username)
	if err != nil {
		if err.Error() == "недостаточно средств" {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	return c.Send(fmt.Sprintf("Пользователь @%s размучен за %d зеток. Ваш текущий баланс: %d зеток.", strings.Trim(username, "@"), amount, balance))
}
