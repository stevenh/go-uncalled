package uncalled_test

import (
	"testing"

	"github.com/stevenh/go-uncalled/pkg/uncalled"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(
		t,
		testdata,
		uncalled.NewAnalyzer(
			uncalled.TestWriter(t),
		),
		"./database/sql/rows/err",
		"./net/http/request/body/close",
	)
}
