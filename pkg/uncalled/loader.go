package uncalled

import (
	"sync/atomic"

	"golang.org/x/tools/go/analysis"
	"gopkg.in/yaml.v3"
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
	a := &analyzer{
		cfg:     l.cfg,
		options: l.options,
		log: l.log.
			With().
			Int32("id", l.id.Add(1)).
			Logger(),
	}

	return a.run(pass)
}

// String implements flag.Value.
func (l *loader) String() string {
	b, _ := yaml.Marshal(l.cfg)
	return string(b)
}

// String implements flag.Value.
func (l *loader) Set(file string) error {
	l.cfg = &Config{}
	return l.cfg.load(file)
}
