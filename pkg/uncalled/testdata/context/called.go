package uncalled_test

import (
	"context"
	"fmt"
	"os"
)

func Called() {
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	fmt.Fprintf(os.Stderr, "cancel: %p\n", cancel)
}
