package logger

import (
	"os"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	globalLogger *zap.SugaredLogger
	currentLevel int32 = int32(LevelInfo)
)

type Config struct {
	Level  string
	Format string // "text" (default) or "json"
}

func Init(cfg Config) error {
	parsedLevel, err := ParseLevel(cfg.Level)
	if err != nil {
		return err
	}
	atomic.StoreInt32(&currentLevel, int32(parsedLevel))

	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "json" {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zapcore.Level(parsedLevel),
	)

	globalLogger = zap.New(core, zap.AddCaller()).Sugar()
	return nil
}

func ParseLevel(raw string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, newUnsupportedLevelError(raw)
	}
}

func newUnsupportedLevelError(raw string) error {
	return &unsupportedLevelError{level: raw}
}

type unsupportedLevelError struct {
	level string
}

func (e *unsupportedLevelError) Error() string {
	return "unsupported log level: " + e.level
}

func Debugf(format string, args ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelDebug {
		globalLogger.Debugf(format, args...)
	}
}

func Infof(format string, args ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelInfo {
		globalLogger.Infof(format, args...)
	}
}

func Warnf(format string, args ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelWarn {
		globalLogger.Warnf(format, args...)
	}
}

func Errorf(format string, args ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelError {
		globalLogger.Errorf(format, args...)
	}
}

func Debugw(msg string, keysAndValues ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelDebug {
		globalLogger.Debugw(msg, keysAndValues...)
	}
}

func Infow(msg string, keysAndValues ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelInfo {
		globalLogger.Infow(msg, keysAndValues...)
	}
}

func Warnw(msg string, keysAndValues ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelWarn {
		globalLogger.Warnw(msg, keysAndValues...)
	}
}

func Errorw(msg string, keysAndValues ...any) {
	if globalLogger == nil {
		return
	}
	if Level(atomic.LoadInt32(&currentLevel)) <= LevelError {
		globalLogger.Errorw(msg, keysAndValues...)
	}
}

func Sync() error {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.Sync()
}
