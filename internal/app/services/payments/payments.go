package payments

import (
	"errors"
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

func (s Service) Pay(from string, to string, amount int) (int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, err
	}

	if dataFrom["balance"].(int) < amount {
		return dataFrom["balance"].(int), errors.New("недостаточно средств")
	}

	if dataTo["id"].(int64) == 0 {
		return dataFrom["balance"].(int), errors.New("пользователь не зарегистрирован в боте (пусть напишет что-либо в чат)")
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int)-amount)
	if err != nil {
		return 0, err
	}
	_, err = s.User.SetUserBalance(dataTo["id"].(int64), dataTo["balance"].(int)+amount)
	if err != nil {
		return 0, err
	}

	return balance, nil
}
