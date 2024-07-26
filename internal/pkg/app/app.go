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
	"log"
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
		logger.Error("ошибка при инициализации кэша: ", zap.Error(err))
		return nil, err
	}

	err = db.Init(cfg.DB.DBUser, cfg.DB.DBPassword, cfg.DB.DBHost, cfg.DB.DBName)
	if err != nil {
		logger.Fatal("ошибка при инициализации БД: ", zap.Error(err))
		return nil, err
	}

	a := &App{}

	InitBot(cfg.TelegramAPI, a)

	return a, nil
}

func InitBot(TelegramAPI string, a *App) {
	pref := tele.Settings{
		Token:  TelegramAPI,
		Poller: &tele.LongPoller{Timeout: 1 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		logger.Error("бот")
	}

	a.users = usersService.New()
	a.payments = paymentsService.New(a.users)
	a.mutes = mutesService.New(a.users)
	a.plays = playsService.New(a.users)

	users := users.Endpoint{User: a.users}
	mwUsers := middleware.Endpoint{Bot: b, User: a.users}
	payments := payments.Endpoint{Payment: a.payments}
	mutes := mutes.Endpoint{Mute: a.mutes}
	plays := plays.Endpoint{Play: a.plays}

	b.Use(mwUsers.IsUser)

	b.Handle("/help", func(c tele.Context) error {
		c.Send("🚀 Базовые команды\n" +
			"/user <username> - Посмотреть информацию о пользователе\n" +
			"/pay <username> <amount> - Перевести необходимую сумму пользователю\n" +
			"/mute <username> <duration> - Замутить пользователя на какое-то количество времени (формат - 5s/11m/23h)\n" +
			"/unmute <username> - Размутить пользователя\n\n" +
			"🎰 Мини-игры\n" +
			"/slots <amount> - Сыграть в казино (коэффициенты от x2 до x100❗️)\n" +
			"/steal <username> <amount> - Украсть сумму у пользователя (чем больше сумма, тем ниже шанс)")
		return nil
	})

	b.Handle("/user", users.GetUserData)
	b.Handle("/pay", payments.PayHandler)
	b.Handle("/mute", mutes.MuteHandler)
	b.Handle("/unmute", mutes.UnmuteHandler)
	b.Handle("/slots", plays.SlotsHandler)
	b.Handle("/steal", plays.StealHandler)

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

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Запуск функции сразу при запуске приложения
		if err := a.users.IncrementAllUserBalances(); err != nil {
			log.Fatalf("не удалось обновить баланс всех пользователей: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				err := a.users.IncrementAllUserBalances()
				if err != nil {
					log.Printf("не удалось обновить баланс всех пользователей: %v", err)
				} else {
					log.Println("баланс всех пользователей успешно обновлен")
				}
			}
		}
	}()

	b.Start()
}
