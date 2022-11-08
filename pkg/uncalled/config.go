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
	// DefaultCategory is the default category used to report rules
	// which don't specify one.
	DefaultCategory string `yaml:"default-category" mapstructure:"default-category"`

	// Rules are the rules to process, disabled rules will be skipped.
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
	// Name is the name of the rule.
	Name string

	// Disable disables the processing of this rule if set to true.
	Disabled bool

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

// build returns an error if r isn't valid, nil otherwise.
func (r *Rule) validate() error {
	if len(r.Packages) == 0 {
		return fmt.Errorf("rule %q: no packages", r.Name)
	}

	if len(r.Results) == 0 {
		return fmt.Errorf("rule %q: no call results", r.Name)
	}

	for i, res := range r.Results {
		if res.Expect != nil {
			if r.expects != nil {
				return fmt.Errorf("rule %q: more than one result expecting a method", r.Name)
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

	if len(r.expectedCalls) == 0 {
		return fmt.Errorf("rule %q: no interested call results", r.Name)
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
			return fmt.Errorf("rule: %q is expected and wildcard %q", rule.Name, r.Type)
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
