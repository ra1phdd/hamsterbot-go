package payments

import (
	"errors"
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

func (s Service) Pay(to string, from string, amount int) (int64, error) {
	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, err
	}

	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return 0, err
	}

	balanceTo := dataTo["balance"].(int64)
	balanceFrom := dataFrom["balance"].(int64)

	if balanceFrom < int64(amount) {
		return balanceFrom, errors.New("недостаточно средств")
	}

	if dataTo["id"].(int64) == 0 {
		return balanceFrom, errors.New("пользователь не зарегистрирован")
	}

	balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), balanceFrom-int64(amount))
	if err != nil {
		return 0, err
	}
	_, err = s.User.SetUserBalance(dataTo["id"].(int64), balanceTo+int64(amount))
	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (s Service) PayAdm(to string, amount int) (int64, error) {
	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return 0, err
	}

	balanceTo := dataTo["balance"].(int64)

	if dataTo["id"].(int64) == 0 {
		return balanceTo, errors.New("пользователь не зарегистрирован")
	}

	_, err = s.User.SetUserBalance(dataTo["id"].(int64), balanceTo+int64(amount))
	if err != nil {
		return balanceTo, err
	}

	return balanceTo, nil
}
