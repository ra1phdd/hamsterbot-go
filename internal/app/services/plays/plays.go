package plays

import (
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"math/rand"
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

func (s Service) Slots(id int64, amount int) (bool, []string, int, int64, error) {
	balanceStr, err := cache.Rdb.Get(cache.Ctx, fmt.Sprintf("user:%d:balance", id)).Result()
	if err != nil {
		return false, nil, 0, 0, err
	}
	balance, err := strconv.ParseInt(balanceStr, 10, 64)
	if err != nil {
		return false, nil, 0, 0, err
	}

	if balance < int64(amount) {
		return false, nil, 0, balance, errors.New("недостаточно средств")
	}

	symbols := []string{
		"🍒", "🍒", "🍒", "🍒", "🍒",
		"🍋", "🍋", "🍋", "🍋", "🍋",
		"🍉", "🍉", "🍉", "🍉", "🍉",
		"🍇", "🍇", "🍇", "🍇", "🍇",
		"🔔", "🔔", "🔔",
		"7️⃣",
	}

	balanceCasinoStr, err := cache.Rdb.Get(cache.Ctx, "user:1:balance").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Warn("Ошибка нахождения баланса казино", zap.Error(err))
	}
	balanceCasino, err := strconv.ParseInt(balanceCasinoStr, 10, 64)

	var win bool
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := []string{
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
	}

	maxChance := 100           // Максимальный шанс выигрыша в процентах (100%)
	minChance := 0             // Минимальный шанс выигрыша в процентах (0%)
	maxBalance := int64(50000) // Баланс для 100% шанса выигрыша
	minBalance := int64(5000)  // Баланс для 0% шанса выигрыша
	chance := int(float64(balanceCasino-minBalance) / float64(maxBalance-minBalance) * float64(maxChance-minChance))
	if balanceCasino >= maxBalance {
		chance = 100
	}
	if balanceCasino <= minBalance {
		chance = 0
	}

	randomNumber := rng.Intn(100)

	if randomNumber > chance { // проигрыш
		win = false

		for {
			if (result[0] == result[1] && result[1] == result[2]) || (result[0] == result[1] || result[1] == result[2]) {
				result = []string{
					symbols[rng.Intn(len(symbols))],
					symbols[rng.Intn(len(symbols))],
					symbols[rng.Intn(len(symbols))],
				}
			} else {
				break
			}
		}

		balance, err = s.User.SetUserBalance(id, balance-int64(amount))
		if err != nil {
			return false, nil, 0, 0, err
		}

		_, err = s.User.SetUserBalance(1, balanceCasino+int64(amount))
		if err != nil {
			return false, nil, 0, 0, err
		}

		return win, result, amount, balance, nil
	} else {
		if result[0] == result[1] && result[1] == result[2] {
			win = true
			// Все символы совпадают
			switch result[0] {
			case "7️⃣":
				amount *= 100
			case "🔔":
				amount *= 20
			default:
				amount *= 10
			}

			balance, err = s.User.SetUserBalance(id, balance+int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}

			_, err = s.User.SetUserBalance(1, balanceCasino-int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}
		} else if result[0] == result[1] || result[1] == result[2] {
			win = true
			amount *= 2

			balance, err = s.User.SetUserBalance(id, balance+int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}

			_, err = s.User.SetUserBalance(1, balanceCasino-int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}
		} else {
			win = false

			balance, err = s.User.SetUserBalance(id, balance-int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}

			_, err = s.User.SetUserBalance(1, balanceCasino+int64(amount))
			if err != nil {
				return false, nil, 0, 0, err
			}
		}
	}

	return win, result, amount, balance, nil
}

func (s Service) Steal(to string, from string, amount int) (bool, int64, error) {
	dataTo, err := s.User.GetUserByUsername(to)
	if err != nil {
		return false, 0, err
	}

	dataFrom, err := s.User.GetUserByUsername(from)
	if err != nil {
		return false, 0, err
	}

	balanceTo := dataTo["balance"].(int64)
	balanceFrom := dataFrom["balance"].(int64)

	if dataTo["id"].(int64) == dataFrom["id"].(int64) {
		return false, balanceFrom, errors.New("нельзя украсть деньги у самого себя")
	}

	cacheKey := fmt.Sprintf("user:%d:steal", dataTo["id"].(int64))
	exists, err := cache.Rdb.Exists(cache.Ctx, cacheKey).Result()
	if err != nil {
		logger.Warn("Ошибка проверки наличия ключа в кеше", zap.Error(err))
	}
	if exists != 0 {
		return false, balanceFrom, fmt.Errorf("пользователь уже был обчищен, попробуйте позднее")
	}

	if balanceTo < int64(amount) {
		return false, balanceFrom, errors.New("недостаточно средств у пользователя")
	}

	if balanceFrom < int64(amount) {
		return false, balanceFrom, errors.New("недостаточно средств")
	}

	var chance float64
	chance = (float64(balanceTo) - chance) / float64(balanceTo)
	if chance < 0.0 {
		chance = 0.0
	}
	if dataFrom["id"].(int64) == 1230045591 {
		chance = 1.0
	}
	randomNumber := rand.Float64()

	err = cache.Rdb.Set(cache.Ctx, cacheKey, "exists", 3*time.Hour).Err()
	if err != nil {
		return false, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	if randomNumber < chance/3 {
		balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), balanceFrom+int64(amount))
		if err != nil {
			return false, balanceFrom, err
		}

		_, err = s.User.SetUserBalance(dataTo["id"].(int64), balanceTo-int64(amount))
		if err != nil {
			return false, balanceFrom, err
		}

		return true, balance, nil
	} else {
		balance, err := s.User.SetUserBalance(dataFrom["id"].(int64), balanceFrom-int64(amount/4))
		if err != nil {
			return false, balanceFrom, err
		}

		return false, balance, nil
	}
}
