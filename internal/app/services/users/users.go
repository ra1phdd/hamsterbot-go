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
	"log"
	"strconv"
	"strings"
	"time"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s Service) GetUserById(id int64) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("user:%d", id)
	fields := []string{"username", "balance", "lvl", "income", "mute", "selfmute"}
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
		case "mute", "selfmute":
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
		"selfmute": models.Mute{},
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
		} else if field == "mute" || field == "selfmute" {
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
		"selfmute": models.Mute{},
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
		} else if field == "mute" || field == "selfmute" {
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

func (s Service) GetUserBalance(id int64) (int64, error) {
	balanceStr, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("user:%d:balance", id)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	balance, err := strconv.ParseInt(balanceStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return balance, nil
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

func (s Service) GetTopByBalance() ([]models.UserTop, error) {
	var top []models.UserTop
	cacheValue, err := cache.Rdb.Get(cache.Ctx, "top:balance").Result()
	if err == nil && cacheValue != "" {
		err = json.Unmarshal([]byte(cacheValue), &top)
		if err != nil {
			log.Printf("Ошибка десериализации: %v", err)
		}
		return top, nil
	} else if !errors.Is(err, redis.Nil) {
		// Если ошибка не связана с отсутствием ключа, логируем её
		log.Printf("Ошибка при получении данных из Redis: %v", err)
	}

	rows, err := db.Conn.Query(`SELECT username, balance FROM users WHERE username NOT LIKE 'bank%' ORDER by balance DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.UserTop
		errScan := rows.Scan(&user.Username, &user.Value)
		if errScan != nil {
			return nil, errScan
		}

		top = append(top, user)
	}

	cacheValueByte, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	err = cache.Rdb.Set(cache.Ctx, "top:balance", cacheValueByte, 5*time.Minute).Err()
	if err != nil {
		log.Printf("Ошибка при сохранении данных в Redis: %v", err)
	}

	return top, nil
}

func (s Service) GetTopByLVL() ([]models.UserTop, error) {
	var top []models.UserTop
	cacheValue, err := cache.Rdb.Get(cache.Ctx, "top:lvl").Result()
	if err == nil && cacheValue != "" {
		err = json.Unmarshal([]byte(cacheValue), &top)
		if err != nil {
			log.Printf("Ошибка десериализации: %v", err)
		}
		return top, nil
	} else if !errors.Is(err, redis.Nil) {
		// Если ошибка не связана с отсутствием ключа, логируем её
		log.Printf("Ошибка при получении данных из Redis: %v", err)
	}

	rows, err := db.Conn.Query(`SELECT username, lvl FROM users WHERE username NOT LIKE 'bank%' ORDER by lvl DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.UserTop
		errScan := rows.Scan(&user.Username, &user.Value)
		if errScan != nil {
			return nil, errScan
		}

		top = append(top, user)
	}

	cacheValueByte, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	err = cache.Rdb.Set(cache.Ctx, "top:lvl", cacheValueByte, 5*time.Minute).Err()
	if err != nil {
		log.Printf("Ошибка при сохранении данных в Redis: %v", err)
	}

	return top, nil
}

func (s Service) GetTopByIncome() ([]models.UserTop, error) {
	var top []models.UserTop
	cacheValue, err := cache.Rdb.Get(cache.Ctx, "top:income").Result()
	if err == nil && cacheValue != "" {
		err = json.Unmarshal([]byte(cacheValue), &top)
		if err != nil {
			log.Printf("Ошибка десериализации: %v", err)
		}
		return top, nil
	} else if !errors.Is(err, redis.Nil) {
		// Если ошибка не связана с отсутствием ключа, логируем её
		log.Printf("Ошибка при получении данных из Redis: %v", err)
	}

	rows, err := db.Conn.Query(`SELECT username, income FROM users WHERE username NOT LIKE 'bank%' ORDER by income DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.UserTop
		errScan := rows.Scan(&user.Username, &user.Value)
		if errScan != nil {
			return nil, errScan
		}

		top = append(top, user)
	}

	cacheValueByte, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	err = cache.Rdb.Set(cache.Ctx, "top:income", cacheValueByte, 5*time.Minute).Err()
	if err != nil {
		log.Printf("Ошибка при сохранении данных в Redis: %v", err)
	}

	return top, nil
}

func (s Service) GetBankBalance() (map[string]interface{}, error) {
	bankBalance, err := s.GetUserById(1)
	if err != nil {
		return nil, err
	}

	var usersBankBalance int64
	query := `SELECT SUM(balance) FROM users WHERE username LIKE 'bank_%'`
	err = db.Conn.QueryRowx(query).Scan(&usersBankBalance)
	if err != nil {
		logger.Error("ошибка при выборке данных из таблицы users в функции getUserData", zap.Error(err))
		return nil, err
	}

	return map[string]interface{}{
		"bank":  bankBalance["balance"].(int64),
		"users": usersBankBalance,
	}, nil
}
