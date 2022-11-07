package uncalled

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/types"
	"strings"
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

// Rule represents an individual rule for uncalled Analyzer.
type Rule struct {
	Name     string
	Disabled bool
	Severity string
	Packages []string
	Call     Call
	Expect   Expect

	types map[string]struct{}
}

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

	r.types = make(map[string]struct{})
	for _, res := range r.Call.Results {
		if err := res.build(r); err != nil {
			return err
		}
	}

	if len(r.types) == 0 {
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
func (r *Result) build(rule *Rule) error {
	matches := make(map[string]struct{}, len(rule.Packages))
	for i, p := range rule.Packages {
		name := r.name(p)
		if i == rule.Expect.ResultIndex {
			if r.Type == anyType {
				return fmt.Errorf("rule: %q is interesting and wildcard %q", rule.Name, r.Type)
			}
			rule.types[name] = struct{}{}
		}
		matches[name] = struct{}{}
	}

	r.match = func(t types.Type) bool {
		if t == nil {
			return false
		}

		if r.Type == anyType {
			return true // Matches any type.
		}

		_, ok := matches[t.String()]
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
	ResultIndex int `yaml:"resultIndex" mapstructure:"resultIndex"`
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
