package uncalled

import (
	"fmt"

	"github.com/rs/zerolog"
)

// log is a command line configurable logger.
type log struct {
	zerolog.Logger
}

// String implements flag.Value.
func (l *log) String() string {
	return l.GetLevel().String()
}

// String implements flag.Value.
func (l *log) Set(val string) error {
	if val == "true" {
		if lvl := l.Logger.GetLevel(); lvl > zerolog.TraceLevel {
			l.Logger = l.Level(lvl - 1)
		}
		return nil
	}

	lvl, err := zerolog.ParseLevel(val)
	if err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	l.Logger = l.Level(lvl)

	return nil
}

// IsBoolFlag is an optional method which indicates this is bool flag.
func (l log) IsBoolFlag() bool {
	return true
}
