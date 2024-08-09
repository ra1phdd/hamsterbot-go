package mutes

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"hamsterbot/internal/app/models"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"regexp"
	"strconv"
	"time"
)

type User interface {
	GetUserByUsername(username string) (map[string]interface{}, error)
	SetUserBalance(id int64, balance int64) (int64, error)
}

type Service struct {
	User User
}

func New(User User) *Service {
	return &Service{
		User: User,
	}
}

func GetDuration(durationStr string) (time.Duration, error) {
	logger.Debug("Получение длительности мута", zap.String("duration", durationStr))

	re := regexp.MustCompile(`^(\d+)([smh])$`)
	matches := re.FindStringSubmatch(durationStr)
	if matches == nil {
		return 0, errors.New("неизвестная единица времени (1s/2m/3h)")
	}
	logger.Debug("Выходные данные от регулярного выражения", zap.Any("matches", matches))

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}

	var duration time.Duration
	switch matches[2] {
	case "s":
		duration = time.Duration(value) * time.Second
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	default:
		return 0, errors.New("неизвестная единица времени (1s/2m/3h)")
	}

	logger.Debug("Вычисленная длительность", zap.Any("duration", duration))

	return duration, nil
}

func GetAmount(typeMute string, duration time.Duration) (int, error) {
	logger.Debug("Получение стоимости мута", zap.String("type", typeMute), zap.Any("duration", duration))

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

	var amount int
	switch {
	case duration%time.Hour == 0:
		amount = int(duration.Seconds()) * ratioHour
	case duration%time.Minute == 0:
		amount = int(duration.Seconds()) * ratioMinute
	case duration%time.Second == 0:
		amount = int(duration.Seconds()) * ratioSecond
	default:
		amount = int(duration.Seconds()) * ratioHour
	}

	logger.Debug("Вычисленная длительность и стоимость с учетом коэффициента", zap.Any("duration", duration), zap.Int("amount", amount))

	return amount, nil
}

func (s Service) Mute(to string, from string, durationStr string) (int64, int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, 0, err
	}

	duration, err := GetDuration(durationStr)
	if err != nil {
		return 0, 0, err
	}

	amount, err := GetAmount("mute", duration)
	if err != nil {
		return 0, 0, err
	}

	if dataFrom["balance"].(int64) < int64(amount) {
		logger.Info("У пользователя недостаточно средств", zap.Any("from", dataFrom))
		return dataFrom["balance"].(int64), amount, errors.New("недостаточно средств")
	}

	var mute models.Mute
	cacheKey := fmt.Sprintf("user:%d:mute", dataTo["id"].(int64))
	jsonMute, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, err
	}

	if jsonMute != "" {
		err = json.Unmarshal([]byte(jsonMute), &mute)
		if err != nil {
			return 0, 0, err
		}

		if mute != (models.Mute{}) {
			jsonStartMute, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", mute.StartMute)
			if err != nil {
				return 0, 0, err
			}
			jsonDuration := time.Duration(mute.Duration)

			startMute := time.Now().UTC()
			oldDuration := startMute.Sub(jsonStartMute)

			jsonDuration -= oldDuration
			jsonDuration += duration
			jsonStartMute = startMute

			mute.StartMute = fmt.Sprint(jsonStartMute)
			mute.Duration = int64(jsonDuration)
		} else {
			mute.StartMute = fmt.Sprint(time.Now().UTC())
			mute.Duration = int64(duration)
		}
	} else {
		mute.StartMute = fmt.Sprint(time.Now().UTC())
		mute.Duration = int64(duration)
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int64)-int64(amount))
	if err != nil {
		return 0, 0, err
	}

	strMute, err := json.Marshal(mute)
	err = cache.Rdb.Set(cache.Ctx, cacheKey, strMute, time.Duration(mute.Duration)).Err()
	if err != nil {
		return 0, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	return balance, amount, nil
}

func (s Service) Unmute(from string, to string) (int64, int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, 0, err
	}

	var mute models.Mute
	cacheKey := fmt.Sprintf("user:%d:mute", dataTo["id"].(int64))
	jsonMute, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, err
	}

	if jsonMute != "" {
		err = json.Unmarshal([]byte(jsonMute), &mute)
		if err != nil {
			return 0, 0, err
		}

		if mute != (models.Mute{}) {
			jsonStartMute, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", mute.StartMute)
			if err != nil {
				return 0, 0, err
			}
			jsonDuration := time.Duration(mute.Duration)

			currentTime := time.Now().UTC()
			duration := currentTime.Sub(jsonStartMute)

			mute.StartMute = ""
			mute.Duration = int64(jsonDuration - duration)
		} else {
			return 0, 0, fmt.Errorf("пользователь не в муте")
		}
	} else {
		return 0, 0, fmt.Errorf("пользователь не в муте")
	}

	amount, err := GetAmount("unmute", time.Duration(mute.Duration))
	if err != nil {
		return 0, 0, err
	}

	if dataFrom["balance"].(int64) < int64(amount) {
		logger.Info("У пользователя недостаточно средств", zap.Any("from", dataFrom))
		return dataFrom["balance"].(int64), amount, errors.New("недостаточно средств")
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int64)-int64(amount))
	if err != nil {
		return 0, 0, err
	}

	err = cache.Rdb.Del(cache.Ctx, cacheKey).Err()
	if err != nil {
		return 0, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	return balance, amount, nil
}
