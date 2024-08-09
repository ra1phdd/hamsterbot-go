package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"hamsterbot/internal/app/models"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/db"
	"hamsterbot/pkg/logger"
	"strconv"
	"strings"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s Service) GetUserById(id int64) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("user:%d", id)
	fields := []string{"username", "balance", "lvl", "income", "mute"}
	data := map[string]interface{}{"id": id}
	var err error

	for _, field := range fields {
		cacheValue, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("%s:%s", cacheKey, field)).Result()
		if (err != nil && !errors.Is(err, redis.Nil)) || (cacheValue == "" && field != "mute") {
			data = nil
			break
		}

		switch field {
		case "username":
			data[field] = cacheValue
		case "balance", "lvl", "income":
			value, convErr := strconv.ParseInt(cacheValue, 10, 64)
			if convErr != nil {
				return nil, convErr
			}
			data[field] = value
		case "mute":
			var mute models.Mute
			if cacheValue != "" {
				err := json.Unmarshal([]byte(cacheValue), &mute)
				if err != nil {
					return nil, err
				}
			}

			data[field] = mute
		}
	}

	if data != nil {
		return data, nil
	}

	var username string
	var balance, lvl, income int64
	query := `SELECT username, balance, lvl, income FROM users WHERE id = $1`
	err = db.Conn.QueryRowx(query, id).Scan(&username, &balance, &lvl, &income)
	if err != nil {
		logger.Error("ошибка при выборке данных из таблицы users в функции getUserData", zap.Error(err))
		return nil, err
	}

	data = map[string]interface{}{
		"id":       id,
		"username": username,
		"balance":  balance,
		"lvl":      lvl,
		"income":   income,
		"mute":     models.Mute{},
	}

	err = cache.Rdb.Set(cache.Ctx, fmt.Sprintf("username:%s", username), id, 0).Err()
	if err != nil {
		return nil, err
	}
	for field, value := range data {
		if field == "id" {
			continue
		} else if field == "username" {
			value = strings.Trim(username, "@")
		} else if field == "mute" {
			value, err = json.Marshal(models.Mute{})
			if err != nil {
				return nil, err
			}
		}

		err = cache.Rdb.Set(cache.Ctx, fmt.Sprintf("%s:%s", cacheKey, field), value, 0).Err()
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (s Service) GetUserByUsername(username string) (map[string]interface{}, error) {
	var data map[string]interface{}

	idStr, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("username:%s", strings.Trim(username, "@"))).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if !errors.Is(err, redis.Nil) {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, err
		}

		data, err = s.GetUserById(id)
		if err != nil {
			return nil, err
		}

		return data, err
	}

	var id, balance, lvl, income int64
	query := `SELECT id, balance, lvl, income FROM users WHERE username = $1`
	err = db.Conn.QueryRowx(query, strings.Trim(username, "@")).Scan(&id, &balance, &lvl, &income)
	if err != nil {
		logger.Error("ошибка при выборке данных из таблицы users в функции getUserData", zap.Error(err))
		return nil, fmt.Errorf("пользователь не найден")
	}

	data = map[string]interface{}{
		"id":       id,
		"username": strings.Trim(username, "@"),
		"balance":  balance,
		"lvl":      lvl,
		"income":   income,
		"mute":     models.Mute{},
	}

	cacheKey := fmt.Sprintf("user:%d", id)
	err = cache.Rdb.Set(cache.Ctx, fmt.Sprintf("username:%s", strings.Trim(username, "@")), id, 0).Err()
	if err != nil {
		return nil, err
	}
	for field, value := range data {
		if field == "id" {
			continue
		} else if field == "username" {
			value = strings.Trim(username, "@")
		} else if field == "mute" {
			value, err = json.Marshal(models.Mute{})
			if err != nil {
				return nil, err
			}
		}

		err = cache.Rdb.Set(cache.Ctx, fmt.Sprintf("%s:%s", cacheKey, field), value, 0).Err()
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (s Service) SetUserBalance(id int64, balance int64) (int64, error) {
	rows, err := db.Conn.Queryx(`UPDATE users SET balance = $1 WHERE id = $2`, balance, id)
	if err != nil {
		logger.Error("ошибка при добавлении пользователя в таблицу users", zap.Error(err))
		return 0, err
	}
	defer rows.Close()

	err = cache.Rdb.Set(cache.Ctx, fmt.Sprintf("user:%d:balance", id), balance, 0).Err()
	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (s Service) IncrementAllUserBalances() error {
	keys, err := cache.Rdb.Keys(cache.Ctx, "user:*:balance").Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	for _, key := range keys {
		idStr := strings.Trim(strings.Trim(key, "user:"), ":balance")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return err
		}

		balanceStr, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("user:%s:balance", idStr)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		balance, err := strconv.ParseInt(balanceStr, 10, 64)
		if err != nil {
			return err
		}

		incomeStr, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("user:%s:income", idStr)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		income, err := strconv.ParseInt(incomeStr, 10, 64)
		if err != nil {
			return err
		}

		_, err = s.SetUserBalance(id, balance+income)
		if err != nil {
			return fmt.Errorf("ошибка при обновлении баланса: %w", err)
		}
	}

	return nil
}

func (s Service) AddUser(id int64, username string) error {
	rows, err := db.Conn.Queryx(`INSERT INTO users (id, username, balance, lvl, income) VALUES ($1, $2, 1500, 1, 250)`, id, username)
	if err != nil {
		logger.Error("ошибка при добавлении пользователя в таблицу users", zap.Error(err))
		return err
	}
	defer rows.Close()

	return nil
}
