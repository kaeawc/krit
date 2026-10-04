package serve

import (
	"fmt"
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/testutil"
)

func TestMain(m *testing.M) {
	cleanup, err := testutil.IsolateDaemons()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
