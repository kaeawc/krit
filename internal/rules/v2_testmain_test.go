package rules_test

import (
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

func TestMain(m *testing.M) {
	// Bridge v2 rules into the v1 Registry so tests that look up rules
	// by name can find rules registered via v2.Register.
	rules.RegisterV2Rules()
	os.Exit(m.Run())
}
