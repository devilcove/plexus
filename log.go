package plexus

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func SetLogging(v string) *slog.Logger {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	logLevel := &slog.LevelVar{}
	replace := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.SourceKey {
			source, ok := a.Value.Any().(*slog.Source)
			if ok {
				source.File = filepath.Base(source.File)
				source.Function = filepath.Base(source.Function)
			}
		}
		if a.Key == slog.TimeKey {
			a.Value = slog.StringValue(time.Now().Format(time.DateTime))
		}

		return a
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{AddSource: true, ReplaceAttr: replace, Level: logLevel}))
	slog.SetDefault(logger)
	switch v {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "INFO":
		logLevel.Set(slog.LevelInfo)
	case "WARN":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.Debug("Logging level set to", "level", logLevel.Level())
	return logger
}
