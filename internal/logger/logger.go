package logger

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var currentLevel int32 = int32(LevelInfo)

func Init(level string) error {
	parsed, err := ParseLevel(level)
	if err != nil {
		return err
	}
	atomic.StoreInt32(&currentLevel, int32(parsed))
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
		return LevelInfo, fmt.Errorf("unsupported log level: %s", raw)
	}
}

func Debugf(format string, args ...any) {
	logf(LevelDebug, "DEBUG", format, args...)
}

func Infof(format string, args ...any) {
	logf(LevelInfo, "INFO", format, args...)
}

func Warnf(format string, args ...any) {
	logf(LevelWarn, "WARN", format, args...)
}

func Errorf(format string, args ...any) {
	logf(LevelError, "ERROR", format, args...)
}

func logf(level Level, tag, format string, args ...any) {
	if level < Level(atomic.LoadInt32(&currentLevel)) {
		return
	}
	log.Printf("["+tag+"] "+format, args...)
}
