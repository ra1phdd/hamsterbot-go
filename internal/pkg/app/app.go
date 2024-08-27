package app

import (
	"fmt"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"hamsterbot/config"
	"hamsterbot/internal/app/endpoint/mutes"
	"hamsterbot/internal/app/endpoint/payments"
	"hamsterbot/internal/app/endpoint/plays"
	"hamsterbot/internal/app/endpoint/users"
	"hamsterbot/internal/app/middleware"
	mutesService "hamsterbot/internal/app/services/mutes"
	paymentsService "hamsterbot/internal/app/services/payments"
	playsService "hamsterbot/internal/app/services/plays"
	usersService "hamsterbot/internal/app/services/users"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/db"
	"hamsterbot/pkg/logger"
	"hamsterbot/pkg/metrics"
	"log"
	"strings"
	"time"
)

type App struct {
	users    *usersService.Service
	payments *paymentsService.Service
	mutes    *mutesService.Service
	plays    *playsService.Service
}

func New() (*App, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Ошибка при попытке спарсить .env файл в структуру: %v", err)
	}

	logger.Init(cfg.LoggerLevel)

	err = cache.Init(fmt.Sprintf("%s:%s", cfg.Redis.RedisAddr, cfg.Redis.RedisPort), cfg.Redis.RedisUsername, cfg.Redis.RedisPassword, cfg.Redis.RedisDBId)
	if err != nil {
		logger.Fatal("ошибка при инициализации кэша: ", zap.Error(err))
		return nil, err
	}

	err = db.Init(cfg.DB.DBUser, cfg.DB.DBPassword, cfg.DB.DBHost, cfg.DB.DBName)
	if err != nil {
		logger.Fatal("ошибка при инициализации БД: ", zap.Error(err))
		return nil, err
	}

	go metrics.Init()

	a := &App{}

	InitBot(cfg.TelegramAPI, a)

	return a, nil
}

func InitBot(TelegramAPI string, a *App) {
	botLogger := logger.Named("bot")
	pref := tele.Settings{
		Token:  TelegramAPI,
		Poller: &tele.LongPoller{Timeout: 1 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		botLogger.Fatal("Ошибка при создании бота", zap.Error(err))
	}

	go func() {
		ubLogger := logger.Named("updateBalance")

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := a.users.IncrementAllUserBalances(); err != nil {
					ubLogger.Error("ошибка обновления баланса пользователей", zap.Error(err))
				} else {
					ubLogger.Info("баланс пользователей успешно обновлен")
				}
			}
		}
	}()

	a.users = usersService.New()
	a.payments = paymentsService.New(a.users)
	a.mutes = mutesService.New(a.users)
	a.plays = playsService.New(a.users, a.mutes)

	mwEndpoint := middleware.Endpoint{Bot: b, User: a.users}
	usersEndpoint := users.Endpoint{User: a.users}
	paymentsEndpoint := payments.Endpoint{Payment: a.payments, User: a.users}
	mutesEndpoint := mutes.Endpoint{Mute: a.mutes}
	playsEndpoint := plays.Endpoint{Play: a.plays}

	b.Use(mwEndpoint.IsUser)

	b.Handle("/help", func(c tele.Context) error {
		err := c.Send("🚀 Базовые команды\n" +
			"/user <username> - Посмотреть информацию о пользователе\n" +
			"/pay <username> <amount> - Перевести необходимую сумму пользователю\n" +
			"/mute <username> <duration> - Замутить пользователя на какое-то количество времени (формат - 5s/11m/23h)\n" +
			"/unmute <username> - Размутить пользователя\n\n" +
			"🎰 Мини-игры\n" +
			"/slots <amount> - Сыграть в казино (коэффициенты от x2 до x100 ❗)")
		if err != nil {
			return err
		}
		return nil
	})
	//b.Handle("/rule", playsEndpoint.Rules)

	// user команды
	b.Handle("/user", usersEndpoint.GetUserData)
	//b.Handle("/top", usersEndpoint.TopHandler)
	//b.Handle("/topb", func(c tele.Context) error {
	//	return usersEndpoint.TopHandlerCommand(c, "balance")
	//})
	//b.Handle("/topl", func(c tele.Context) error {
	//	return usersEndpoint.TopHandlerCommand(c, "lvl")
	//})
	//b.Handle("/topi", func(c tele.Context) error {
	//	return usersEndpoint.TopHandlerCommand(c, "income")
	//})
	b.Handle("/bank", paymentsEndpoint.BankHandler)
	b.Handle("/pay", paymentsEndpoint.PayHandler)
	b.Handle("/mute", mutesEndpoint.MuteHandler)
	b.Handle("/unmute", mutesEndpoint.UnmuteHandler)
	b.Handle("/slots", playsEndpoint.SlotsHandler)
	//b.Handle("/rln", playsEndpoint.RouletteNumHandler)
	//b.Handle("/rlc", playsEndpoint.RouletteColorHandler)
	//b.Handle("/dice", playsEndpoint.DiceHandler)
	//b.Handle("/rsp", playsEndpoint.RockPaperScissorsHandler)
	//b.Handle("/selfmute", playsEndpoint.SelfMuteHandler)
	//b.Handle("/selfunmute", playsEndpoint.SelfUnmuteHandler)
	b.Handle("/steal", playsEndpoint.StealHandler)

	// adm команды
	b.Handle("/payd", paymentsEndpoint.PayAdmHandler)
	b.Handle("/send", func(c tele.Context) error {
		if c.Sender().ID != 1230045591 {
			return nil
		}

		args := c.Args()

		chatID := int64(-1002138316635)

		// Используем метод Send у объекта бота для отправки сообщения
		_, err := c.Bot().Send(tele.ChatID(chatID), strings.Join(args, " "))
		return err
	})

	// обработчики для всех типов сообщений
	// необходимо для того, чтобы правильно работал
	// middleware (удалял сообщения в случае мута и прочее)
	b.Handle(tele.OnText, func(c tele.Context) error { return nil })
	b.Handle(tele.OnAudio, func(c tele.Context) error { return nil })
	b.Handle(tele.OnCallback, func(c tele.Context) error { return nil })
	b.Handle(tele.OnDocument, func(c tele.Context) error { return nil })
	b.Handle(tele.OnEdited, func(c tele.Context) error { return nil })
	b.Handle(tele.OnMedia, func(c tele.Context) error { return nil })
	b.Handle(tele.OnPhoto, func(c tele.Context) error { return nil })
	b.Handle(tele.OnSticker, func(c tele.Context) error { return nil })
	b.Handle(tele.OnVideo, func(c tele.Context) error { return nil })
	b.Handle(tele.OnVoice, func(c tele.Context) error { return nil })

	b.Start()
}
