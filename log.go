package plexus

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var LoggingLevel = new(slog.LevelVar)

func SetUpLogging(v string) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     LoggingLevel,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if attr.Key == slog.SourceKey {
				if s, _ := attr.Value.Any().(*slog.Source); s != nil {
					s.File = filepath.Base(s.File)
				}
			}
			return attr
		},
	})))
	SetLogging(v)
}

func SetLogging(v string) {
	switch strings.ToUpper(v) {
	case "DEBUG":
		LoggingLevel.Set(slog.LevelDebug)
	case "INFO":
		LoggingLevel.Set(slog.LevelInfo)
	case "WARN":
		LoggingLevel.Set(slog.LevelWarn)
	case "ERROR":
		LoggingLevel.Set(slog.LevelError)
	default:
		LoggingLevel.Set(slog.LevelInfo)
	}
	slog.Info("set log level", "level", LoggingLevel)
}
