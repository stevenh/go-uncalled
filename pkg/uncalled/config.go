package uncalled

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	anyType = "_"
)

var (
	// reName is pattern which validates rule names.
	reName = regexp.MustCompile("^[a-z0-9-]+$")
)

//go:embed .uncalled.yaml
var defaultConfig []byte

// quote quotes s.
func quote(s string) string {
	if strconv.CanBackquote(s) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// loadDefaultConfig loads the default embedded configuration.
func loadDefaultConfig() (*Config, error) {
	cfg := &Config{}
	if err := cfg.load(bytes.NewBuffer(defaultConfig)); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", quote(string(defaultConfig)), err)
	}

	return cfg, nil
}

// Config represents the configuration for uncalled Analyzer.
type Config struct {
	// DisableAll disables all rules.
	DisableAll bool `yaml:"disable-all" mapstructure:"disable-all"`

	// Disabled disables the given rules.
	Disabled []string

	// Enabled enables specific rules, in combination with disable all.
	Enabled []string

	// Rules are the rules to process, disabled rules will be skipped.
	Rules []Rule

	// rules lists all rules and their index in Rules.
	rules map[string]Rule

	// active lists active rules.
	active map[string]Rule
}

// loadFile loads the analyzer config from file.
func (c *Config) loadFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		// No file in wrap as that's in err already.
		return fmt.Errorf("load config: %w", err)
	}
	defer f.Close()

	return c.load(f)
}

// load loads the analyzer config from r.
func (c *Config) load(r io.Reader) error {
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(c); err != nil {
		return fmt.Errorf("decode config: %q: %w", "file", err)
	}

	return c.validate()
}

// string returns a YAML string representation of c.
// If an error occurs it is returned instead.
func (c *Config) string() string {
	s, err := c.yaml()
	if err != nil {
		return err.Error()
	}

	return string(s)
}

// yaml returns a yaml representation of c.
func (c *Config) yaml() ([]byte, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return b, nil
}

// copy returns a deep copy of c.
func (c *Config) copy() (*Config, error) {
	// Leverage yaml serialisation to create a deep copy.
	data, err := c.yaml()
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := cfg.load(bytes.NewBuffer(data)); err != nil {
		return nil, err
	}

	// Validate after copy to ensure we don't cause any data races.
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// merge merges other into c if not nil.
func (c *Config) merge(other *Config) error {
	if other == nil {
		return nil
	}

	c.DisableAll = other.DisableAll
	c.Disabled = other.Disabled
	c.Enabled = other.Enabled

	for _, otherRule := range other.Rules {
		rule, ok := c.rules[otherRule.Name]
		if ok {
			// Existing rule overwrite.
			c.Rules[rule.idx] = otherRule
		} else {
			// New rule append
			c.Rules = append(c.Rules, otherRule)
		}
	}

	return c.validate()
}

// validate validates the configuration.
func (c *Config) validate() error {
	c.active = make(map[string]Rule)
	c.rules = make(map[string]Rule)
	disabled := make(map[string]struct{})

	for i, r := range c.Rules {
		if err := r.validate(); err != nil {
			return err
		}

		r.idx = i
		if !c.DisableAll {
			c.active[r.Name] = r
		}
		c.rules[r.Name] = r
	}

	for _, r := range c.Disabled {
		if _, ok := c.rules[r]; !ok {
			return fmt.Errorf("rule %q: in disabled unknown", r)
		}
		disabled[r] = struct{}{}
		delete(c.active, r)
	}

	for _, name := range c.Enabled {
		r, ok := c.rules[name]
		if !ok {
			return fmt.Errorf("rule %q: in enabled unknown", name)
		}

		if _, ok := disabled[name]; ok {
			return fmt.Errorf("rule %q: in both enabled and disabled", name)
		}
		c.active[name] = r
	}

	return nil
}

// Rule represents an individual rule for uncalled Analyzer.
type Rule struct {
	// Name is the name of the rule.
	Name string

	// Category is the category used to report failures for this rule.
	Category string

	// Packages is the list of package imports which to be considered
	// When processing this rule. If one of the listed packages isn't
	// imported by the code being checked the rule is automatically
	// skipped. At least one package must be specified.
	Packages []string

	// Call represents the call to match to trigger rule processing.
	// Methods is a list of method calls on the package which trigger
	// the rule to be checked.
	// TODO: Implemented.
	Methods []string

	// Results represents the results the matched methods return.
	Results []*Result

	// idx represents the index at which this rule was in Config.Rules.
	idx int

	// expects references the result which specifies a Method.
	expects *Result

	// expected calls is a map of fully qualified calls we expect.
	expectedCalls map[string]struct{}

	// expectedType is a map of fully qualifed types to monitor.
	expectedTypes map[string]struct{}
}

// name returns the expected string based on ident.
func (r Rule) name(ident string) string {
	if ident == "" {
		ident = strings.TrimLeft(r.expects.Type, ".")
	}

	return fmt.Sprintf("%s%s(%s)", ident, r.expects.Expect.Call, strings.Join(r.expects.Expect.Args, ","))
}

// validate returns an error if r isn't valid, nil otherwise.
func (r *Rule) validate() error {
	switch {
	case !reName.MatchString(r.Name):
		return fmt.Errorf("rule %q: contains non alpha numberic or uppercase charaters", r.Name)
	case len(r.Packages) == 0:
		return fmt.Errorf("rule %q: no packages", r.Name)
	case len(r.Results) == 0:
		return fmt.Errorf("rule %q: no call results", r.Name)
	}

	for i, res := range r.Results {
		if res.Expect != nil {
			if r.expects != nil {
				return fmt.Errorf("rule %q: multiple results expecting a method", r.Name)
			}
			res.idx = i
			r.expects = res
		}
	}

	if r.expects == nil {
		return fmt.Errorf("rule %q: no result expecting a method", r.Name)
	}

	r.expectedCalls = make(map[string]struct{})
	r.expectedTypes = make(map[string]struct{})
	for _, res := range r.Results {
		if err := res.build(r); err != nil {
			return err
		}
	}

	return nil
}

// matchesResults returns true if the res matches the Results of this rule,
// false otherwise.
func (r *Rule) matchesResults(res *types.Tuple) bool {
	if res.Len() != len(r.Results) {
		return false // Function results length does match.
	}

	for i, r := range r.Results {
		if !r.match(res.At(i).Type()) {
			return false
		}
	}

	return true
}

// matchesCall returns true if call and name match the expected call for
// this rule, false otherwise.
func (r *Rule) matchesCall(call *ast.CallExpr, name string) bool {
	if len(call.Args) != len(r.expects.Expect.Args) {
		return false
	}

	if strings.HasPrefix(r.expects.Expect.Call, ".") {
		return r.expects.Expect.Call[1:] == name
	}

	return r.expects.Expect.Call == name
}

// Result is a result expected from a rule call.
type Result struct {
	// Type is name of the type.
	// If "_" then matches any type.
	// If prefixed by "." then matches any type of specified
	// in its Rule.Packages.
	Type string

	// Pointer specifies if this type should be a pointer to
	// the named TypeName.
	Pointer bool

	// Expect sets the expectation for the result.
	// At least one Result in a rule must have a method specified.
	// If not specified no check it performed.
	Expect *Expect

	idx   int
	match resultMatcher
}

// resultMatcher is a function which returns true if t matches, false otherwise.
type resultMatcher func(t types.Type) bool

// build builds the matcher for this result.
func (r *Result) build(rule *Rule) error {
	resultTypes := make(map[string]struct{}, len(rule.Packages))
	for _, p := range rule.Packages {
		name := r.name(p)
		resultTypes[name] = struct{}{}
		if r.Expect == nil {
			continue // Not expecting a method on this result to be called.
		}

		// Expected result type.
		if r.Type == anyType {
			return fmt.Errorf("rule %q: result idx %d is expected and wildcard", rule.Name, r.idx)
		}

		name += r.Expect.Call
		rule.expectedCalls[name] = struct{}{}

		parts := strings.Split(name, ".")
		name = strings.Join(parts[:len(parts)-1], ".")
		rule.expectedTypes[name] = struct{}{}
	}

	r.match = func(t types.Type) bool {
		if t == nil {
			return false
		}

		if r.Type == anyType {
			return true // Matches any type.
		}

		_, ok := resultTypes[t.String()]
		return ok
	}

	return nil
}

// name returns the fully qualified type name for given pkg.
func (r Result) name(pkg string) string {
	if r.Type == anyType {
		return ""
	}

	var ptr string
	if r.Pointer {
		ptr = "*"
	}

	if strings.HasPrefix(r.Type, ".") {
		return fmt.Sprintf("%s%s%s", ptr, pkg, r.Type)
	}

	return fmt.Sprintf("%s%s", ptr, r.Type)
}

// Expect represents a result call expectation.
type Expect struct {
	// Call is the call to expect on this result.
	// Methods called on the result should start with a "."
	// for example .Err
	Call string

	// Args are the arguments passed to the method.
	// Currently on the count matters.
	Args []string
}
