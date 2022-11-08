package uncalled

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	anyType = "_"
)

//go:embed .uncalled.yaml
var defaultConfig []byte

// Config represents the configuration for uncalled Analyzer.
type Config struct {
	Rules []Rule
}

// loadConfig loads the analyzer config from file.
func (c *Config) load(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	if err := dec.Decode(c); err != nil {
		return fmt.Errorf("decode config: %q: %w", file, err)
	}

	return nil
}

// Rule represents an individual rule for uncalled Analyzer.
type Rule struct {
	Name     string
	Disabled bool
	Severity string
	Packages []string
	Call     Call
	Expect   Expect

	expectedCalls map[string]struct{}
	expectedTypes map[string]struct{}
}

// expects returns the expected string based on ident.
func (r Rule) expects(ident string) string {
	if ident == "" {
		ident = strings.TrimLeft(r.Call.Results[r.Expect.ResultIndex].Type, ".")
	}

	return fmt.Sprintf("%s%s(%s)", ident, r.Expect.Method, strings.Join(r.Expect.Args, ","))
}

// build returns an error if r isn't valid, nil otherwise.
func (r *Rule) validate() error {
	if len(r.Packages) == 0 {
		return fmt.Errorf("rule %q: no packages", r.Name)
	}

	if len(r.Call.Results) == 0 {
		return fmt.Errorf("rule %q: no call results", r.Name)
	}

	if r.Expect.ResultIndex > len(r.Call.Results) {
		return fmt.Errorf("rule %q: invalid call index %d for %d results", r.Name, r.Expect.ResultIndex, len(r.Call.Results))
	}

	r.expectedCalls = make(map[string]struct{})
	r.expectedTypes = make(map[string]struct{})
	for i, res := range r.Call.Results {
		if err := res.build(r, i); err != nil {
			return err
		}
	}

	if len(r.expectedCalls) == 0 {
		return fmt.Errorf("rule %q: no interested call results", r.Name)
	}

	return nil
}

// Call represents the call that results in the return a rule
// checks for calls on.
type Call struct {
	Methods []string
	Results []*Result
}

func (c Call) matches(res *types.Tuple) bool {
	if res.Len() != len(c.Results) {
		return false // Function results length does match.
	}

	for i, r := range c.Results {
		if !r.match(res.At(i).Type()) {
			return false
		}
	}

	return true
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

	match resultMatcher
}

// resultMatcher is a function which returns true if t matches, false otherwise.
type resultMatcher func(t types.Type) bool

// build builds the matcher for this result.
func (r *Result) build(rule *Rule, idx int) error {
	resultTypes := make(map[string]struct{}, len(rule.Packages))
	for _, p := range rule.Packages {
		name := r.name(p)
		resultTypes[name] = struct{}{}
		if idx != rule.Expect.ResultIndex {
			continue
		}

		// Expected result type.
		if r.Type == anyType {
			return fmt.Errorf("rule: %q is expected and wildcard %q", rule.Name, r.Type)
		}

		name += rule.Expect.Method
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

// Expect is the expected call for a Rule.
type Expect struct {
	Method      string
	ResultIndex int `yaml:"result-index" mapstructure:"result-index"`
	Args        []string
}

// matches returns ture if call and name match this Expect, false otherwise.
func (e Expect) matches(call *ast.CallExpr, name string) bool {
	if len(call.Args) != len(e.Args) {
		return false
	}

	if strings.HasPrefix(e.Method, ".") {
		return e.Method[1:] == name
	}

	return e.Method == name
}
