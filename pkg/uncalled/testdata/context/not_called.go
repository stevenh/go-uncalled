package uncalled_test

import (
	"context"
	"fmt"
	"os"
)

func NotCalled() {
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx) // want "cancel\\(\\) must be called"
	fmt.Fprintf(os.Stderr, "cancel: %p\n", cancel)
}
