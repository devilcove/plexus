package plexus

import (
	"log"
	"log/slog"
	"strings"
)

func SetLogging(v string) {
	log.SetFlags(log.Lshortfile) // journald adds timestamp
	switch strings.ToUpper(v) {
	case "DEBUG":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	case "INFO":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "WARN":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "ERROR":
		slog.SetLogLoggerLevel(slog.LevelError)
	default:
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}
}
