package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"hamsterbot/pkg/db"
	"hamsterbot/pkg/logger"
	"log"
)

var (
	Ctx = context.Background()
	Rdb *redis.Client
)

type Queue struct {
	Query string
	Args  []Args
}

type Args struct {
	Value string
}

func Init(Addr string, Username string, Password string, DB int) error {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     Addr,
		Username: Username,
		Password: Password,
		DB:       DB,
	})

	err := Rdb.Ping(Ctx).Err()
	if err != nil {
		return err
	}

	return nil
}

func InsertCache(query string, args []Args) error {
	var insert []Queue
	cacheValue, err := Rdb.Get(Ctx, "cache:insert").Result()
	if err == nil && cacheValue != "" {
		err = json.Unmarshal([]byte(cacheValue), &insert)
		if err != nil {
			log.Printf("Ошибка десериализации: %v", err)
		}
		return nil
	} else if !errors.Is(err, redis.Nil) {
		// Если ошибка не связана с отсутствием ключа, логируем её
		log.Printf("Ошибка при получении данных из Redis: %v", err)
	}

	newInsert := Queue{
		Query: query,
		Args:  args,
	}
	insert = append(insert, newInsert)

	cacheValueByte, err := json.Marshal(insert)
	if err != nil {
		return err
	}
	err = Rdb.Set(Ctx, "cache:insert", cacheValueByte, 0).Err()
	if err != nil {
		log.Printf("Ошибка при сохранении данных в Redis: %v", err)
	}

	return nil
}

func AddToDB() error {
	var insert []Queue
	cacheValue, err := Rdb.Get(Ctx, "cache:insert").Result()
	if err == nil && cacheValue != "" {
		err = json.Unmarshal([]byte(cacheValue), &insert)
		if err != nil {
			log.Printf("Ошибка десериализации: %v", err)
		}
		return nil
	} else if !errors.Is(err, redis.Nil) {
		// Если ошибка не связана с отсутствием ключа, логируем её
		log.Printf("Ошибка при получении данных из Redis: %v", err)
	}

	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}

	// В случае ошибки откатываем транзакцию
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			logger.Error("Ошибка применения insert-значений в БД", zap.Error(p.(error)))
		} else if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, i := range insert {
		rows, err := tx.Query(i.Query, i.Args)
		if err != nil {
			return err
		}
		defer rows.Close()
	}

	if err = Rdb.Del(Ctx, "cache:insert").Err(); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func ClearCacheByPattern(pattern string) error {
	keys, err := Rdb.Keys(Ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	// Удаление всех ключей
	if len(keys) > 0 {
		if err := Rdb.Del(context.Background(), keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

func ClearCache(Rdb *redis.Client) error {
	// Удаление всего кэша из Redis
	err := Rdb.FlushAll(Ctx).Err()
	if err != nil {
		return err
	}
	return nil
}
