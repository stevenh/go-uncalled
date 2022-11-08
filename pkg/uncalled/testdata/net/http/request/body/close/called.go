package uncalled_test

import (
	"fmt"
	"io"
	"net/http"
)

func Called() {
	resp, err := http.Get("http://example.com/")
	if err != nil {
		// Handle error.
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Handle error.
	}
	fmt.Println(string(body))
}
