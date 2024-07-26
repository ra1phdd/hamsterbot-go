package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"log"
)

type Configuration struct {
	TelegramAPI string `env:"TELEGRAM_API,required"`
	LoggerLevel string `env:"LOGGER_LEVEL" envDefault:"debug"`
	DB          DB
	Redis       Redis
}

type DB struct {
	DBUser     string `env:"DB_USER,required"`
	DBPassword string `env:"DB_PASSWORD,required"`
	DBName     string `env:"DB_NAME,required"`
	DBHost     string `env:"DB_HOST,required"`
}

type Redis struct {
	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPort     string `env:"REDIS_PORT" envDefault:"6379"`
	RedisUsername string `env:"REDIS_USERNAME,required"`
	RedisPassword string `env:"REDIS_PASSWORD,required"`
	RedisDBId     int    `env:"REDIS_DB_ID,required"`
}

func NewConfig(files ...string) (*Configuration, error) {
	err := godotenv.Load(files...)
	if err != nil {
		log.Fatalf("Файл .env не найден: %s", err)
	}

	cfg := Configuration{}
	err = env.Parse(&cfg)
	if err != nil {
		return nil, err
	}
	err = env.Parse(&cfg.Redis)
	if err != nil {
		return nil, err
	}
	err = env.Parse(&cfg.DB)

	return &cfg, nil
}
