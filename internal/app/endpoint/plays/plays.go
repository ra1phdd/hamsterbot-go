package plays

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
	"hamsterbot/internal/app/constants"
	"hamsterbot/pkg/logger"
	"strconv"
	"strings"
)

type Play interface {
	Slots(id, amount int64) (bool, bool, []string, int64, int64, error)
	RouletteNum(id, number, amount int64) (bool, bool, int64, int64, int64, error)
	RouletteColor(id, color, amount int64) (bool, bool, string, int64, int64, error)
	Dice(id, number, amount int64) (bool, bool, []int64, int64, int64, error)
	RockPaperScissors(id, number, amount int64) (bool, bool, string, int64, int64, error)
	Steal(to string, from string, amount int) (bool, int64, error)
	SelfMute(id int64, durationStr string) (int64, int64, error)
	SelfUnmute(id int64) (int64, int64, error)
}

type Endpoint struct {
	Play Play
}

func (e *Endpoint) Rules(c telebot.Context) error {
	args := c.Args()

	if len(args) == 1 {
		switch args[0] {
		case "slots":
			return c.Send("В игре 'Слоты' игрок выбирает ставку в зетках, которую он хочет сделать. " +
				"После этого случайным образом выпадают три символа.\n\n\t•\tЕсли 2 из 3 выпавших символа " +
				"совпадают, игрок получает x2 суммы ставки.\n\t•\tЕсли 3 выпавших символа сопадают, то" +
				"игрок получает x100 за 7️⃣, x20 за 🔔 и x10 за остальные символы суммы ставки.\n\t•\tЕсли совпадений " +
				"нет, то ставка считается проигранной.\n\nПример команды: /slots 100")
		case "rln":
			return c.Send("В игре 'Рулетка по числу' игрок выбирает число от 1 до 36 и ставку в зетках.\n\n\t•\t" +
				"Если выпавшее число совпадает с выбранным числом, игрок выигрывает и получает x35 суммы ставки. " +
				"В противном случае, ставка считается проигранной.\n\nПример команды: /rln 36 100")
		case "rlc":
			return c.Send("В игре 'Рулетка по цвету' игрок выбирает цвет (черный, красный или зеленый) и ставку в " +
				"зетках.\n\n\t•\tЕсли выпавший цвет совпадает с выбранным, игрок выигрывает и получает x2 суммы ставки" +
				"\n\t•\tВ противном случае, ставка считается проигранной.\n\nПример команды: /rlc ч 100")
		case "dice":
			return c.Send("В игре 'Кости' игрок выбирает сумму ставки и предполагаемую сумму двух кубиков " +
				"(от 2 до 12).\n\n\t•\tЕсли сумма чисел на кубиках совпадает с предполагаемой, игрок выигрывает " +
				"и получает x12 суммы ставки \n\t•\tВ противном случае, ставка считается проигранной.\n\nПример " +
				"команды: /dice 12 100")
		case "rsp":
			return c.Send("В игре 'Камень-ножницы-бумага' игрок выбирает камень/ножницы/бумагу и ставку в " +
				"зетках.\n\n\t•\tЕсли выбор игрока совпадает с выбором компьютера, игрок выигрывает и получает x3 суммы " +
				"ставки\n\t•\tВ противном случае, ставка считается проигранной.\n\nПример команды: /rsp к 100")
		default:
			return c.Send("Неизвестная команда")
		}
	}
	return c.Send("Неизвестная команда")
}

func (e *Endpoint) SlotsHandler(c telebot.Context) error {
	var amount int64
	args := c.Args()

	if len(args) == 1 { // /slots <amount>
		var err error
		amount, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return err
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /slots <сумма>.")
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	win, autoloss, result, newAmount, balance, err := e.Play.Slots(c.Sender().ID, amount)
	if err != nil {
		if err.Error() == constants.ErrLackBalance {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\n%s | %s | %s\n\n", amount, result[0], result[1], result[2])
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", newAmount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) играет в слоты", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Bool("autoloss", autoloss), zap.Any("result", result),
		zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) StealHandler(c telebot.Context) error {
	var username string
	var amount int64
	var err error
	args := c.Args()

	if len(args) == 2 { // /steal <username> <сумма>
		username = args[0]
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /steal username 100")
		}
	} else if len(args) == 1 && c.Message().ReplyTo != nil { // /steal <сумма>
		username = c.Message().ReplyTo.Sender.Username
		amount, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте правильный формат, например: /steal username 100")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /steal username сумма или ответьте командой /steal сумма на сообщение.")
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	win, balance, err := e.Play.Steal(username, c.Sender().Username, int(amount))
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Попытка украсть %d зеток у @%s: ", amount, strings.Trim(username, "@"))
	if win {
		resultMsg += fmt.Sprintf("✅ Успешно! \n\n Ваш баланс: %d зеток\n", balance)
	} else {
		resultMsg += fmt.Sprintf("🚫 Неудача( \n\n Ваш баланс: %d зеток\n", balance)
	}

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) попытался украсть деньги у пользователя @%s", c.Sender().Username, c.Sender().ID, strings.Trim(username, "@")),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) RouletteNumHandler(c telebot.Context) error {
	var amount, num int64
	args := c.Args()

	if len(args) == 2 {
		var err error
		num, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /rln 36 100.")
		}
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /rln 36 100.")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /rln <число> <сумма>.")
	}

	if num < 1 || num > 36 {
		return c.Send("Ошибка: " + constants.ErrNegativeRln)
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	win, autoloss, result, newAmount, balance, err := e.Play.RouletteNum(c.Sender().ID, num, amount)
	if err != nil {
		if err.Error() == constants.ErrLackBalance {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\nВыпавшее число: %d\n\n", amount, result)
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", newAmount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) играет в рулетку по числу", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Bool("autoloss", autoloss), zap.Any("result", result),
		zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) RouletteColorHandler(c telebot.Context) error {
	var amount, color int64
	var colorStr string
	args := c.Args()

	if len(args) == 2 {
		colorStr = args[0]

		var err error
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /rlc ч 100")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /rlc цвет(ч/к/з) сумма.")
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	switch colorStr {
	case "ч", "черный", "черное", "чёрное", "чёрный", "black":
		color = 1
	case "к", "кр", "красное", "красный", "red":
		color = 2
	case "з", "зеленое", "зелёное", "зел", "green":
		color = 3
	default:
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /rlc цвет(ч/к/з) сумма.")
	}

	win, autoloss, result, newAmount, balance, err := e.Play.RouletteColor(c.Sender().ID, color, amount)
	if err != nil {
		if err.Error() == constants.ErrLackBalance {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\nВыпавший цвет: %s\n\n", amount, result)
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", newAmount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) играет в рулетку по цвету", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Bool("autoloss", autoloss), zap.Any("result", result),
		zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) DiceHandler(c telebot.Context) error {
	var amount, num int64
	args := c.Args()

	if len(args) == 2 {
		var err error
		num, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /dice 11 100.")
		}
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /dice 11 100.")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /dice <число> <сумма>.")
	}

	if num < 2 || num > 12 {
		return c.Send("Ошибка: число должно находиться в диапазоне от 2 до 12.")
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	win, autoloss, result, newAmount, balance, err := e.Play.Dice(c.Sender().ID, num, amount)
	if err != nil {
		if err.Error() == constants.ErrLackBalance {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\nНа 🎲№1 выпало: %d\nНа 🎲№2 выпало: %d\n\n", amount, result[0], result[1])
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", newAmount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) играет в кости", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Bool("autoloss", autoloss), zap.Any("result", result),
		zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) RockPaperScissorsHandler(c telebot.Context) error {
	var amount, choice int64
	var choiceStr string
	args := c.Args()

	if len(args) == 2 {
		choiceStr = args[0]

		var err error
		amount, err = strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return c.Send("Неверный формат суммы. Пожалуйста, используйте: /rlc ч 100")
		}
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /rlc цвет(ч/к/з) сумма.")
	}

	if amount < 0 {
		return c.Send("Ошибка: " + constants.ErrNegativeAmount)
	} else if amount < 10 {
		return c.Send("Ошибка: " + constants.ErrLessAmount)
	}

	switch choiceStr {
	case "к", "камень", "rock":
		choice = 1
	case "н", "ножницы", "scissors":
		choice = 2
	case "б", "бумага", "paper":
		choice = 3
	default:
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /rsp к/н/б сумма.")
	}

	win, autoloss, result, newAmount, balance, err := e.Play.RockPaperScissors(c.Sender().ID, choice, amount)
	if err != nil {
		if err.Error() == constants.ErrLackBalance {
			return c.Send(fmt.Sprintf("Ошибка: %s. Ваш текущий баланс: %d зеток.", err.Error(), balance))
		}
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	resultMsg := fmt.Sprintf("🎰 Играем на %d зеток\n\nВыбор компьютера: %s\n\n", amount, result)
	if win {
		resultMsg += fmt.Sprintf("✅ Поздравляю, вы выиграли! Выигрыш составил: %d зеток\n", newAmount)
	} else {
		resultMsg += "🚫 Увы, вы проиграли.\n"
	}
	resultMsg += fmt.Sprintf("Ваш баланс: %d", balance)

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) играет в камень-ножницы-бумага", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Bool("win", win), zap.Bool("autoloss", autoloss), zap.Any("result", result),
		zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(resultMsg)
}

func (e *Endpoint) SelfMuteHandler(c telebot.Context) error {
	var duration string
	args := c.Args()

	if len(args) == 1 { // /selfmute <время>
		duration = args[0]
	} else {
		return c.Send("Неверный формат команды. Пожалуйста, используйте: /selfmute <время>.")
	}

	if duration == "0s" {
		return c.Send("Ошибка: длина мута не может быть меньше 1s.")
	}

	balance, amount, err := e.Play.SelfMute(c.Sender().ID, duration)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) самостоятельно замутил себя", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.String("duration", duration), zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(fmt.Sprintf("Вы замутили себя на %s. За это время вы заработаете %d зеток. Ваш новый баланс: %d зеток", duration, amount, balance))
}

func (e *Endpoint) SelfUnmuteHandler(c telebot.Context) error {
	balance, amount, err := e.Play.SelfUnmute(c.Sender().ID)
	if err != nil {
		return c.Send(fmt.Sprintf("Ошибка: %s.", err.Error()))
	}

	logger.Infof(fmt.Sprintf("Пользователь @%s (%d) досрочно размутил себя", c.Sender().Username, c.Sender().ID),
		c.Chat().ID, c.Chat().Title, zap.Int64("amount", amount), zap.Int64("balance", balance))
	return c.Send(fmt.Sprintf("Вы досрочно размутили себя и потеряли все заработанные в ходе мута зетки (%d). Ваш баланс: %d зеток", amount, balance))
}
