package logger

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func Init(loggerLevel string) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		fmt.Println("Ошибка загрузки часового пояса:", err.Error())
		return
	}

	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.In(location).Format("2006-01-02 15:04:05"))
	}

	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncodeTime = customTimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(config)
	consoleEncoder := zapcore.NewConsoleEncoder(config)

	err = os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		fmt.Println("Ошибка создания папки logs", err.Error())
		return
	}
	logFile, err := os.OpenFile("logs/hamsteryep.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Ошибка создания файла hamsteryepr.log", err.Error())
		return
	}
	writer := zapcore.AddSync(logFile)

	var defaultLogLevel zapcore.Level
	switch loggerLevel {
	case "debug":
		defaultLogLevel = zapcore.DebugLevel
	case "warn":
		defaultLogLevel = zapcore.WarnLevel
	case "error":
		defaultLogLevel = zapcore.ErrorLevel
	case "fatal":
		defaultLogLevel = zapcore.FatalLevel
	case "info":
	default:
		defaultLogLevel = zapcore.InfoLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, defaultLogLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)
	logger = zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))
	defer logger.Sync()
}

func Debug(message string, fields ...zap.Field) {
	logger.Debug(message, fields...)
}

func Debugf(message string, chatId int64, chatName string, fields ...zap.Field) {
	logger.Debug(fmt.Sprintf("[%d/%s] ", chatId, chatName)+message, fields...)
}

func Info(message string, fields ...zap.Field) {
	logger.Info(message, fields...)
}

func Infof(message string, chatId int64, chatName string, fields ...zap.Field) {
	logger.Info(fmt.Sprintf("[%d/%s] ", chatId, chatName)+message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	logger.Warn(message, fields...)
}

func Warnf(message string, chatId int64, chatName string, fields ...zap.Field) {
	logger.Warn(fmt.Sprintf("[%d/%s] ", chatId, chatName)+message, fields...)
}

func Error(message string, fields ...zap.Field) {
	logger.Error(message, fields...)
}

func Errorf(message string, chatId int64, chatName string, fields ...zap.Field) {
	logger.Error(fmt.Sprintf("[%d/%s] ", chatId, chatName)+message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	logger.Fatal(message, fields...)
}

func Fatalf(message string, chatId int64, chatName string, fields ...zap.Field) {
	logger.Fatal(fmt.Sprintf("[%d/%s] ", chatId, chatName)+message, fields...)
}

func Named(s string) *zap.Logger {
	return logger.Named(s)
}
