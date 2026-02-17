package logging

import (
	"context"
	"encoding/json"
	"os"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type LevelSource interface {
	GetLogLevel(ctx context.Context) (string, error)
}

type DBLevelSource struct {
	Pool *pgxpool.Pool
}

func (s *DBLevelSource) GetLogLevel(ctx context.Context) (string, error) {
	var v []byte
	err := s.Pool.QueryRow(ctx, "SELECT value_json FROM config_keys WHERE key = 'log.level'").Scan(&v)
	if err != nil {
		return "", err
	}

	var level string
	if err := json.Unmarshal(v, &level); err != nil {
		return "", err
	}
	return level, nil
}

var currentLevel atomic.Int32 // stores zerolog.Level as int32

func ParseLevel(s string) zerolog.Level {
	switch s {
	case "error":
		return zerolog.ErrorLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "info":
		return zerolog.InfoLevel
	case "debug":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}

func Init() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.ErrorStackMarshaler = func(err error) interface{} { return nil } // optional; see stack section below

	// default
	lvl := zerolog.InfoLevel
	currentLevel.Store(int32(lvl))
	zerolog.SetGlobalLevel(lvl)

	base := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", "gotodo").
		Logger()

	return base
}

func GetLevel() zerolog.Level {
	return zerolog.Level(currentLevel.Load())
}

func SetLevel(lvl zerolog.Level) {
	currentLevel.Store(int32(lvl))
	zerolog.SetGlobalLevel(lvl)
}

func From(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}

func StartLevelRefresher(ctx context.Context, base zerolog.Logger, src LevelSource, every time.Duration) {
	refresh := func() {
		s, err := src.GetLogLevel(ctx)
		if err != nil {
			base.Warn().Err(err).Str("event", "config.loglevel.refresh_failed").Msg("failed to refresh log level")
			return
		}
		lvl := ParseLevel(s)
		old := GetLevel()
		if lvl != old {
			SetLevel(lvl)
			base.Info().
				Str("event", "config.loglevel.changed").
				Str("from", old.String()).
				Str("to", lvl.String()).
				Msg("log level updated")
		}
	}

	refresh()

	t := time.NewTicker(every)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				refresh()
			}
		}
	}()
}
