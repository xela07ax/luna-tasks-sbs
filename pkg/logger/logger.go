package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger инициализирует логгер в зависимости от окружения.
// env может быть "production", "development", "local" и т.д.
func InitLogger(env string) (*zap.Logger, error) {
	if env == "production" {
		// В проде оставляем строгий и быстрый JSON
		return zap.NewProduction()
	}

	// Настройка для локальной разработки (красивый текст)
	encoderConfig := zap.NewDevelopmentEncoderConfig()

	// Делаем уровни цветными для удобного чтения в консоли (DEBUG, WARN, ERROR)
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	// Человекочитаемый формат времени
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Используем ConsoleEncoder вместо JSON
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Настраиваем вывод в stdout с уровнем Debug
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zap.DebugLevel,
	)

	// Собираем логгер.
	// zap.AddCaller() - добавляет файл и строку (например, main.go:42).
	// zap.AddStacktrace(zapcore.ErrorLevel) - включает стек-трейсы ТОЛЬКО начиная с уровня Error,
	// спасая нас от мусора при Warning и Info.
	log := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return log, nil
}
