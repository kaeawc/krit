package oracle_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle/oracletest"
)

// Run the shared Lookup contract against both implementations to
// guarantee the Fake stays in step with the real Oracle on the
// behaviors both populate the same way.

func TestRealOracleSatisfiesLookupContract(t *testing.T) {
	oracletest.RunContract(t, "Oracle", oracletest.LoadFromDataBuilder)
}

func TestFakeOracleSatisfiesLookupContract(t *testing.T) {
	oracletest.RunContract(t, "FakeOracle", oracletest.FakeOracleBuilder)
}
