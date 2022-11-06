package rowserr

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version implements outputing our version information.
type version struct{}

// IsBoolFlag is an optional method which indicates this is bool flag.
func (v version) IsBoolFlag() bool {
	return true
}

// Get implements flag.Getter.
func (v version) Get() interface{} {
	return v.String()
}

// String implements flag.Value.
func (v version) String() string {
	info, _ := debug.ReadBuildInfo()
	return info.Main.Version
}

// String implements flag.Value.
func (v version) Set(string) error {
	fmt.Fprintf(os.Stderr, "%s version %s\n", name, v.String())
	os.Exit(0)
	return nil
}
