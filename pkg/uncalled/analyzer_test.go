package uncalled

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(
		t,
		testdata,
		NewAnalyzer(
			testWriter(t),
		),
		"./context",
		"./database/sql/rows/err",
		"./net/http/request/body/close",
	)
}
