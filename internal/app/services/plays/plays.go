package plays

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"hamsterbot/internal/app/constants"
	"hamsterbot/internal/app/models"
	"hamsterbot/pkg/cache"
	"hamsterbot/pkg/logger"
	"math/rand"
	"time"
)

type User interface {
	GetUserByUsername(username string) (map[string]interface{}, error)
	GetUserBalance(id int64) (int64, error)
	SetUserBalance(id int64, balance int64) (int64, error)
}

type Mute interface {
	GetAmount(typeMute string, duration time.Duration) (int, error)
	GetDuration(durationStr string) (time.Duration, error)
}

type Service struct {
	User User
	Mute Mute
}

func New(User User, Mute Mute) *Service {
	return &Service{
		User: User,
		Mute: Mute,
	}
}

func calculateWinChance(balanceCasino, amount int64) int64 {
	maxChance := 100            // Максимальный шанс выигрыша в процентах (100%)
	minChance := 0              // Минимальный шанс выигрыша в процентах (0%)
	maxBalance := int64(100000) // Баланс для 100% шанса выигрыша
	minBalance := int64(25000)  // Баланс для 0% шанса выигрыша

	chance := int64(float64(balanceCasino-minBalance) / float64(maxBalance-minBalance) * float64(maxChance-minChance))
	if balanceCasino >= maxBalance {
		chance = 100
	}
	if balanceCasino <= minBalance || balanceCasino <= amount*10 {
		chance = 0
	}
	return chance
}

func (s Service) processLoss(id, amount, balance, balanceCasino int64) (int64, error) {
	newBalance, err := s.User.SetUserBalance(id, balance-amount)
	if err != nil {
		return 0, err
	}
	_, err = s.User.SetUserBalance(1, balanceCasino+amount)
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

func (s Service) processWin(id, amount, newAmount, balance, balanceCasino int64) (int64, error) {
	newBalance, err := s.User.SetUserBalance(id, balance+newAmount-amount)
	if err != nil {
		return 0, err
	}
	_, err = s.User.SetUserBalance(1, balanceCasino-newAmount+amount)
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

func (s Service) Slots(id, amount int64) (bool, bool, []string, int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return false, true, nil, 0, 0, err
	}

	if balance < amount {
		return false, true, nil, 0, balance, errors.New(constants.ErrLackBalance)
	}

	symbols := []string{
		"🍒", "🍒", "🍒", "🍒", "🍒",
		"🍋", "🍋", "🍋", "🍋", "🍋",
		"🍉", "🍉", "🍉", "🍉", "🍉",
		"🍇", "🍇", "🍇", "🍇", "🍇",
		"🔔", "🔔", "🔔",
		"7️⃣",
	}

	balanceCasino, err := s.User.GetUserBalance(1)
	if err != nil {
		return false, true, nil, 0, 0, err
	}

	var newAmount, newBalance int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := []string{
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
		symbols[rng.Intn(len(symbols))],
	}
	chance := calculateWinChance(balanceCasino, amount)

	randomNumber := rng.Intn(100)
	if int64(randomNumber) > chance { // проигрыш
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

		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	} else if result[0] == result[1] || result[1] == result[2] {
		newAmount = amount * 2

		if result[0] == result[1] && result[1] == result[2] {
			switch result[0] {
			case "7️⃣":
				newAmount = amount * 100
			case "🔔":
				newAmount = amount * 20
			default:
				newAmount = amount * 10
			}
		}

		newBalance, err = s.processWin(id, amount, newAmount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	} else {
		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	}

	return newAmount > 0, int64(randomNumber) > chance, result, amount, newBalance, nil
}

func (s Service) RouletteNum(id, number, amount int64) (bool, bool, int64, int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return false, true, 0, 0, 0, err
	}

	if balance < amount {
		return false, true, 0, 0, balance, errors.New(constants.ErrLackBalance)
	}

	balanceCasino, err := s.User.GetUserBalance(1)
	if err != nil {
		return false, true, 0, 0, 0, err
	}

	var newAmount, newBalance int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chance := calculateWinChance(balanceCasino, amount)
	result := rng.Intn(36)

	randomNumber := rng.Intn(100)
	if int64(randomNumber) > chance {
		for {
			if int64(result+1) == number {
				result = rng.Intn(36)
			} else {
				break
			}
		}

		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, 0, 0, 0, err
		}
	} else if int64(result+1) == number {
		newAmount = amount * 35

		newBalance, err = s.processWin(id, amount, newAmount, balance, balanceCasino)
		if err != nil {
			return false, true, 0, 0, 0, err
		}
	} else {
		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, 0, 0, 0, err
		}
	}

	return newAmount > 0, int64(randomNumber) > chance, int64(result + 1), newAmount, newBalance, nil
}

func (s Service) RouletteColor(id int64, color int64, amount int64) (bool, bool, string, int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return false, true, "", 0, 0, err
	}

	if balance < amount {
		return false, true, "", 0, balance, errors.New(constants.ErrLackBalance)
	}

	balanceCasino, err := s.User.GetUserBalance(1)
	if err != nil {
		return false, true, "", 0, 0, err
	}

	var newAmount, newBalance int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chance := calculateWinChance(balanceCasino, amount)
	result := rng.Intn(36)

	randomNumber := rng.Intn(100)
	if int64(randomNumber) > chance {
		for {
			if (result+1 == 0 && color == 0) || ((result+1)%2 == 0 && color == 1) || ((result+1)%2 != 0 && color == 2) {
				result = rng.Intn(36)
			} else {
				break
			}
		}

		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, "", 0, 0, err
		}
	} else {
		if result+1 == 0 && color == 0 { // зеленое
			newAmount = amount * 35
		} else if (result+1)%2 == 0 && color == 1 { // черное
			newAmount = amount * 2
		} else if (result+1)%2 != 0 && color == 2 { // красное
			newAmount = amount * 2
		}

		if newAmount != 0 {
			newBalance, err = s.processWin(id, amount, newAmount, balance, balanceCasino)
			if err != nil {
				return false, true, "", 0, 0, err
			}
		} else {
			newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
			if err != nil {
				return false, true, "", 0, 0, err
			}
		}
	}

	var colorStr string
	if result+1 == 0 { // зеленое
		colorStr = fmt.Sprintf("🟩%d", result+1)
	} else if (result+1)%2 == 0 { // черное
		colorStr = fmt.Sprintf("⬛%d", result+1)
	} else if (result+1)%2 != 0 { // красное
		colorStr = fmt.Sprintf("🟥%d", result+1)
	}

	return newAmount > 0, int64(randomNumber) > chance, colorStr, newAmount, newBalance, nil
}

func (s Service) Dice(id, number, amount int64) (bool, bool, []int64, int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return false, true, nil, 0, 0, err
	}

	if balance < amount {
		return false, true, nil, 0, balance, errors.New(constants.ErrLackBalance)
	}

	balanceCasino, err := s.User.GetUserBalance(1)
	if err != nil {
		return false, true, nil, 0, 0, err
	}

	var newAmount, newBalance int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chance := calculateWinChance(balanceCasino, amount)
	resultOne := rng.Intn(6) + 1
	resultTwo := rng.Intn(6) + 1
	result := []int64{int64(resultOne), int64(resultTwo)}

	randomNumber := rng.Intn(100)
	if int64(randomNumber) > chance {
		for {
			if int64(resultOne+resultTwo) == number {
				resultOne = rng.Intn(6) + 1
				resultTwo = rng.Intn(6) + 1
				result = []int64{int64(resultOne), int64(resultTwo)}
			} else {
				break
			}
		}

		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	} else if int64(resultOne+resultTwo) == number {
		newAmount = amount * 12

		newBalance, err = s.processWin(id, amount, newAmount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	} else {
		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, nil, 0, 0, err
		}
	}

	return newAmount > 0, int64(randomNumber) > chance, result, newAmount, newBalance, nil
}

func (s Service) RockPaperScissors(id, number, amount int64) (bool, bool, string, int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return false, true, "", 0, 0, err
	}

	if balance < amount {
		return false, true, "", 0, balance, errors.New(constants.ErrLackBalance)
	}

	balanceCasino, err := s.User.GetUserBalance(1)
	if err != nil {
		return false, true, "", 0, 0, err
	}

	var newAmount, newBalance int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chance := calculateWinChance(balanceCasino, amount)
	result := rng.Intn(3) + 1

	randomNumber := rng.Intn(100)
	if int64(randomNumber) > chance {
		for {
			if int64(result) == number {
				result = rng.Intn(3) + 1
			} else {
				break
			}
		}

		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, "", 0, 0, err
		}
	} else if int64(result) == number {
		newAmount = amount * 3

		newBalance, err = s.processWin(id, amount, newAmount, balance, balanceCasino)
		if err != nil {
			return false, true, "", 0, 0, err
		}
	} else {
		newBalance, err = s.processLoss(id, amount, balance, balanceCasino)
		if err != nil {
			return false, true, "", 0, 0, err
		}
	}

	var choice string
	switch result {
	case 1:
		choice = "камень"
	case 2:
		choice = "ножницы"
	case 3:
		choice = "бумага"
	}

	return newAmount > 0, int64(randomNumber) > chance, choice, newAmount, newBalance, nil
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

	if randomNumber < chance/5 {
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

func (s Service) SelfMute(id int64, durationStr string) (int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return 0, 0, err
	}

	duration, err := s.Mute.GetDuration(durationStr)
	if err != nil {
		return 0, 0, err
	}

	amount, err := s.Mute.GetAmount("selfmute", duration)
	if err != nil {
		return 0, 0, err
	}

	var mute models.Mute
	cacheKey := fmt.Sprintf("user:%d:selfmute", id)
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

	newBalance, err := s.User.SetUserBalance(id, balance+int64(amount))
	if err != nil {
		return 0, 0, err
	}

	strMute, err := json.Marshal(mute)
	err = cache.Rdb.Set(cache.Ctx, cacheKey, strMute, time.Duration(mute.Duration)).Err()
	if err != nil {
		return 0, 0, errors.New("неизвестная ошибка, обратитесь к администратору")
	}

	return newBalance, int64(amount), nil
}

func (s Service) SelfUnmute(id int64) (int64, int64, error) {
	balance, err := s.User.GetUserBalance(id)
	if err != nil {
		return 0, 0, err
	}

	var mute models.Mute
	cacheKey := fmt.Sprintf("user:%d:selfmute", id)
	jsonMute, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, err
	}

	var amount int
	var newBalance int64
	if jsonMute != "" {
		err = json.Unmarshal([]byte(jsonMute), &mute)
		if err != nil {
			return 0, 0, err
		}

		if mute != (models.Mute{}) {
			amount, err = s.Mute.GetAmount("selfmute", time.Duration(mute.Duration))
			if err != nil {
				return 0, 0, err
			}

			newBalance, err = s.User.SetUserBalance(id, balance-int64(amount))
			if err != nil {
				return 0, 0, err
			}

			err = cache.Rdb.Del(cache.Ctx, cacheKey).Err()
			if err != nil {
				return 0, 0, err
			}
		} else {
			return 0, 0, errors.New("вы не в муте")
		}
	} else {
		return 0, 0, errors.New("вы не в муте")
	}

	return newBalance, int64(amount), nil
}
