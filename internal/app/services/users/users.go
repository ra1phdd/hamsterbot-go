package users

import (
	"fmt"
	"go.uber.org/zap"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/db"
	"hamsterbot/pkg/logger"
	"strings"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s Service) GetUserById(id int64) (map[string]interface{}, error) {
	rows, err := db.Conn.Queryx(`SELECT username, balance FROM users WHERE id = $1`, id)
	if err != nil {
		logger.Error("ошибка при выборке данных из таблицы users в функции getUserData", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var username string
	var balance int
	for rows.Next() {
		err := rows.Scan(&username, &balance)
		if err != nil {
			logger.Error("ошибка при обработке данных из таблицы users в функции getUserData", zap.Error(err))
			return nil, err
		}
	}

	data := map[string]interface{}{
		"username": username,
		"balance":  balance,
	}

	return data, nil
}

func (s Service) GetUserByUsername(username string) (map[string]interface{}, error) {
	rows, err := db.Conn.Queryx(`SELECT id, balance FROM users WHERE username = $1`, strings.Trim(username, "@"))
	if err != nil {
		logger.Error("ошибка при выборке данных из таблицы users в функции getUserData", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var id int64
	var balance int
	for rows.Next() {
		err := rows.Scan(&id, &balance)
		if err != nil {
			logger.Error("ошибка при обработке данных из таблицы users в функции getUserData", zap.Error(err))
			return nil, err
		}
	}

	data := map[string]interface{}{
		"id":      id,
		"balance": balance,
	}

	return data, nil
}

func (s Service) SetUserBalance(id int64, balance int) (int, error) {
	rows, err := db.Conn.Queryx(`UPDATE users SET balance = $1 WHERE id = $2`, balance, id)
	if err != nil {
		logger.Error("ошибка при добавлении пользователя в таблицу users", zap.Error(err))
		return 0, err
	}
	defer rows.Close()

	cache.Rdb.Del(cache.Ctx, "GetUserData_"+fmt.Sprint(id))

	return balance, nil
}

func (s Service) IncrementAllUserBalances() error {
	rows, err := db.Conn.Query(`SELECT id, balance FROM users`)
	if err != nil {
		return fmt.Errorf("ошибка при получении списка пользователей: %w", err)
	}
	defer rows.Close()

	var id int64
	var balance int
	for rows.Next() {
		if err := rows.Scan(&id, &balance); err != nil {
			return fmt.Errorf("ошибка при сканировании id пользователя: %w", err)
		}

		_, err = s.SetUserBalance(id, balance+125)
		if err != nil {
			return fmt.Errorf("ошибка при обновлении баланса: %w", err)
		}
	}

	return nil
}

func (s Service) AddUser(id int64, username string) error {
	rows, err := db.Conn.Queryx(`INSERT INTO users (id, username, balance) VALUES ($1, $2, 0)`, id, username)
	if err != nil {
		logger.Error("ошибка при добавлении пользователя в таблицу users", zap.Error(err))
		return err
	}
	defer rows.Close()

	cache.Rdb.Del(cache.Ctx, "GetUserData_"+fmt.Sprint(id))

	return nil
}
