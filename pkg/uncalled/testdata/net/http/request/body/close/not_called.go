package uncalled_test

import (
	"fmt"
	"io"
	"net/http"
)

func NotCalled() {
	resp, err := http.Get("http://example.com/") // want "resp.Body.Close\\(\\) must be called"
	if err != nil {
		// Handle error.
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Handle error.
	}
	fmt.Println(string(body))
}
