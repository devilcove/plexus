package plexus

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lmittmann/tint"
)

const (
	reset  = "\033[0m"
	red    = 31
	yellow = 33
	cyan   = 36
	grey   = 37
)

func color(in string) string {
	switch in {
	case "DEBUG":
		return fmt.Sprintf("\033[%sm%s%s", strconv.Itoa(grey), in, reset)
	case "WARN":
		return fmt.Sprintf("\033[%sm%s%s", strconv.Itoa(yellow), in, reset)
	case "ERROR":
		return fmt.Sprintf("\033[%sm%s%s", strconv.Itoa(red), in, reset)
	case "INFO":
		return fmt.Sprintf("\033[%sm%s%s", strconv.Itoa(cyan), in, reset)
	default:
		return in
	}
}

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
		if a.Key == slog.LevelKey {
			a.Value = slog.StringValue(color(a.Value.String()))
		}
		return a
	}
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		AddSource:  true,
		TimeFormat: time.Kitchen,
		Level:      logLevel,
	}))
	_ = replace
	//logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, ReplaceAttr: replace, Level: logLevel}))
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
