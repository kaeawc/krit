package cache

import (
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
)

func TestMixConfigDataIncludesMarshallableData(t *testing.T) {
	h1 := hashutil.Hasher().New()
	mixConfigData(h1, map[string]interface{}{"k": "v"})

	h2 := hashutil.Hasher().New()
	mixConfigData(h2, map[string]interface{}{"k": "different"})

	if string(h1.Sum(nil)) == string(h2.Sum(nil)) {
		t.Errorf("config data must influence hash; same hash for distinct configs")
	}
}

func TestMixConfigDataMarshalFailureProducesDistinctHash(t *testing.T) {
	// chan is unmarshalable; json.Marshal errors with
	// "json: unsupported type: chan int".
	bad := map[string]interface{}{"k": make(chan int)}

	hBad := hashutil.Hasher().New()
	mixConfigData(hBad, bad)
	badSum := hBad.Sum(nil)

	// Empty / missing-config hash MUST differ from the marshal-error
	// hash. The previous implementation silently dropped the config
	// bytes, making these two states indistinguishable.
	hEmpty := hashutil.Hasher().New()
	mixConfigData(hEmpty, nil)
	if string(badSum) == string(hEmpty.Sum(nil)) {
		t.Errorf("marshal-error hash must differ from no-config hash (silent drop is the bug)")
	}

	// And the marshal-error hash differs from any successfully
	// serialised config — so the cache invalidates if a config
	// changes from marshalable to unmarshalable.
	hOK := hashutil.Hasher().New()
	mixConfigData(hOK, map[string]interface{}{"k": "v"})
	if string(badSum) == string(hOK.Sum(nil)) {
		t.Errorf("marshal-error hash must differ from successfully-marshaled config hash")
	}
}

func TestMixConfigDataMarshalErrorMessageMixedIn(t *testing.T) {
	// The sentinel includes the error message, so two different
	// marshal failures (different bad types) produce different
	// hashes. Otherwise a single sentinel could mask multiple
	// unrelated marshal regressions.
	hChan := hashutil.Hasher().New()
	mixConfigData(hChan, map[string]interface{}{"k": make(chan int)})
	hFunc := hashutil.Hasher().New()
	mixConfigData(hFunc, map[string]interface{}{"k": func() {}})
	if string(hChan.Sum(nil)) == string(hFunc.Sum(nil)) {
		t.Errorf("distinct marshal errors must produce distinct hashes")
	}
}
