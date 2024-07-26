package plays

import (
	"errors"
	"fmt"
	"hamsterbot/pkg/cache"
	"math/rand"
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

func (s Service) Slots(username string, amount int) (bool, []string, int, int, error) {
	data, err := s.User.GetUserByUsername(username)
	if err != nil {
		return false, nil, 0, 0, err
	}

	if data["balance"].(int) < amount {
		return false, nil, 0, data["balance"].(int), errors.New("недостаточно средств")
	}

	balance, err := s.User.SetUserBalance(data["id"].(int64), data["balance"].(int)-amount)
	if err != nil {
		return false, nil, 0, 0, err
	}

	symbols := []string{
		"🍒", "🍒", "🍒", "🍒", "🍒",
		"🍋", "🍋", "🍋", "🍋", "🍋",
		"🍉", "🍉", "🍉", "🍉", "🍉",
		"🍇", "🍇", "🍇", "🍇", "🍇",
		"🔔", "🔔", "🔔",
		"7",
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := []string{
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
	}

	var win bool
	if result[0] == result[1] && result[1] == result[2] {
		win = true
		switch result[0] {
		case "7":
			amount *= 100
		case "🔔":
			amount *= 20
		default:
			amount *= 10
		}

		balance, err = s.User.SetUserBalance(data["id"].(int64), balance+amount)
		if err != nil {
			return false, nil, 0, 0, err
		}
	} else if result[0] == result[1] || result[1] == result[2] {
		win = true
		amount *= 2

		balance, err = s.User.SetUserBalance(data["id"].(int64), balance+amount)
		if err != nil {
			return false, nil, 0, 0, err
		}
	} else {
		win = false
	}

	return win, result, amount, balance, nil
}

func (s Service) Steal(to string, from string, amount int) (bool, int, error) {
	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return false, 0, err
	}

	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return false, 0, err
	}

	cacheKey := fmt.Sprintf("steal:%d", dataFrom["id"].(int64))

	if dataFrom["id"] == dataTo["id"] {
		return false, 0, errors.New("нельзя украсть деньги у себя")
	}

	if dataTo["balance"].(int) < amount/4 {
		return false, 0, errors.New(fmt.Sprintf("недостаточно средств. Ваш баланс: %d зеток", dataTo["balance"].(int)))
	}

	if dataFrom["balance"].(int) < amount {
		return false, 0, errors.New("недостаточно средств у пользователя")
	}

	var chance float64
	chance = (float64(dataFrom["balance"].(int)) - chance) / float64(dataFrom["balance"].(int))
	if chance < 0.0 {
		chance = 0.0
	}
	randomNumber := rand.Float64()

	_, err = cache.Rdb.Set(cache.Ctx, cacheKey, "exists", 3*time.Hour).Result()
	if err != nil {
		return false, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	if randomNumber < chance/5 {
		balance, err := s.User.SetUserBalance(dataTo["id"].(int64), dataTo["balance"].(int)+amount)
		if err != nil {
			return false, dataTo["balance"].(int), err
		}

		_, err = s.User.SetUserBalance(dataFrom["id"].(int64), dataFrom["balance"].(int)-amount)
		if err != nil {
			return false, dataFrom["balance"].(int), err
		}

		return true, balance, nil
	} else {
		balance, err := s.User.SetUserBalance(dataTo["id"].(int64), dataTo["balance"].(int)-(amount/4))
		if err != nil {
			return false, dataTo["balance"].(int), err
		}

		return false, balance, nil
	}
}
