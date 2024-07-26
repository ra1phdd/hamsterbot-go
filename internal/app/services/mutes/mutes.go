package mutes

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"regexp"
	"strconv"
	"time"
)

type User interface {
	GetUserByUsername(username string) (map[string]interface{}, error)
	SetUserBalance(id int64, balance int) (int, error)
}

type Service struct {
	User User
}

func New(User User) *Service {
	return &Service{
		User: User,
	}
}

func GetDurationAndAmount(typeMute string, durationStr string) (time.Duration, int, error) {
	logger.Debug("Получение длительности мута и стоимости", zap.String("type", typeMute), zap.String("duration", durationStr))

	re := regexp.MustCompile(`^(\d+)([smh])$`)
	matches := re.FindStringSubmatch(durationStr)
	if matches == nil {
		return 0, 0, errors.New("неизвестная единица времени (1s/2m/3h)")
	}
	logger.Debug("Выходные данные от регулярного выражения", zap.Any("matches", matches))

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, err
	}

	var ratioSecond, ratioMinute, ratioHour int
	switch typeMute {
	case "mute":
		ratioSecond = 7
		ratioMinute = 5
		ratioHour = 3
	case "unmute":
		ratioSecond = 5
		ratioMinute = 3
		ratioHour = 2
	}

	var duration time.Duration
	var amount int
	switch matches[2] {
	case "s":
		duration = time.Duration(value) * time.Second
		if value > 3600 {
			amount = value * ratioHour
		} else if value > 60 {
			amount = value * ratioMinute
		} else {
			amount = value * ratioSecond
		}
	case "m":
		duration = time.Duration(value) * time.Minute
		if value > 60 {
			amount = value * 60 * ratioHour
		} else {
			amount = value * 60 * ratioMinute
		}
	case "h":
		duration = time.Duration(value) * time.Hour
		amount = value * 60 * 60 * ratioHour
	default:
		return 0, 0, errors.New("неизвестная единица времени (1s/2m/3h)")
	}

	logger.Debug("Вычисленная длительность и стоимость с учетом коэффициента", zap.Any("duration", duration), zap.Int("amount", amount))

	return duration, amount, nil
}

func (s Service) Mute(from string, to string, durationStr string) (int, int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, 0, err
	}

	duration, amount, err := GetDurationAndAmount("mute", durationStr)
	if err != nil {
		return 0, 0, err
	}

	cacheKey := fmt.Sprintf("mute:%d", dataTo["id"].(int64))

	if dataFrom["balance"].(int) < amount {
		logger.Info("У пользователя недостаточно средств", zap.Any("from", dataFrom))
		return dataFrom["balance"].(int), 0, errors.New("недостаточно средств")
	}

	exists, err := cache.Rdb.Exists(cache.Ctx, cacheKey).Result()
	if err != nil {
		logger.Warn("Ошибка проверки наличия ключа в кеше", zap.Error(err))
	}
	if exists != 0 {
		time, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
		if err != nil {
			return 0, 0, err
		}

		logger.Info("Попытка мута при действующем муте", zap.Any("from", dataFrom))
		return 0, 0, errors.New(fmt.Sprintf("пользователь %s уже в муте, блокировка будет снята %s (UTC)", to, time))
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int)-amount)
	if err != nil {
		return 0, 0, err
	}

	currentTime := time.Now().UTC()
	_, err = cache.Rdb.Set(cache.Ctx, cacheKey, currentTime.Add(duration).Format("2006-01-02 15:04:05"), duration).Result()
	if err != nil {
		return 0, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	return balance, amount, nil
}

func (s Service) Unmute(from string, to string) (int, int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, 0, err
	}

	cacheKey := fmt.Sprintf("mute:%d", dataTo["id"].(int64))
	durationStr, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
	if err != nil {
		return 0, 0, err
	}

	currentTime := time.Now().UTC()
	finalDate, err := time.Parse("2006-01-02 15:04:05", durationStr)
	if err != nil {
		return 0, 0, err
	}

	var durStr string
	if finalDate.Sub(currentTime).Hours() >= 1 {
		roundedHours := finalDate.Sub(currentTime).Round(time.Hour).Hours()
		durStr = fmt.Sprintf("%.0fh", roundedHours)
	} else if finalDate.Sub(currentTime).Minutes() >= 1 {
		roundedMinutes := finalDate.Sub(currentTime).Round(time.Minute).Minutes()
		durStr = fmt.Sprintf("%.0fm", roundedMinutes)
	} else {
		roundedSeconds := finalDate.Sub(currentTime).Round(time.Second).Seconds()
		durStr = fmt.Sprintf("%.0fs", roundedSeconds)
	}

	_, amount, err := GetDurationAndAmount("unmute", durStr)
	if err != nil {
		return 0, 0, err
	}

	if dataFrom["balance"].(int) < amount {
		logger.Info("У пользователя недостаточно средств", zap.Any("from", dataFrom))
		return dataFrom["balance"].(int), 0, errors.New("недостаточно средств")
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int)-amount)
	if err != nil {
		return 0, 0, err
	}

	_, err = cache.Rdb.Del(cache.Ctx, cacheKey).Result()
	if err != nil {
		return 0, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	return balance, amount, nil
}
