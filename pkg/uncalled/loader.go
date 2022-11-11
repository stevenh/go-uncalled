package uncalled

import (
	"sync/atomic"

	"golang.org/x/tools/go/analysis"
)

// loader creates a new analyser to process each call to its
// run method, this is needed as analysistest calls run in
// parallel and as analyzer relies on its internal state this
// resulted in random panics.
type loader struct {
	cfg     *Config
	options []Option
	log     log
	id      atomic.Int32
}

// run creates an analyzer and calls run on it.
func (l *loader) run(pass *analysis.Pass) (interface{}, error) {
	// Order of options is important, ours need to go first.
	opts := make([]Option, 0, len(l.options)+2)
	if l.cfg != nil {
		// Take a copy to prevent data races.
		cfg, err := l.cfg.copy()
		if err != nil {
			return nil, err
		}
		opts = append(opts, ConfigOpt(cfg))
	}

	opts = append(opts, logger(l.log.
		With().
		Int32("id", l.id.Add(1)).
		Logger(),
	))

	opts = append(opts, l.options...)

	a, err := newAnalyzer(opts...)
	if err != nil {
		return nil, err
	}

	return a.run(pass)
}

// String implements flag.Value.
func (l *loader) String() string {
	if l.cfg == nil {
		return ""
	}

	return l.cfg.string()
}

// String implements flag.Value.
func (l *loader) Set(file string) error {
	l.cfg = &Config{}
	return l.cfg.loadFile(file)
}
