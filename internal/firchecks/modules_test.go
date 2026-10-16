package firchecks

import (
	"encoding/json"
	"strings"
	"testing"
)

// krit-fir rejects an empty "platform", "kind" or "jvmTarget" if it is ever
// sent as a value, so unset fields must be left out of the request (#759).
func TestModuleSpecOmitsUnsetDefaultedFields(t *testing.T) {
	data, err := json.Marshal(ModuleSpec{ID: ":lib:main", SourceRoots: []string{"/src"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"platform"`, `"kind"`, `"jvmTarget"`} {
		if strings.Contains(string(data), key) {
			t.Errorf("unset %s encoded: %s", key, data)
		}
	}
	data, err = json.Marshal(ModuleSpec{ID: ":app:test", Platform: "android", Kind: "test", JVMTarget: "17"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"platform":"android"`, `"kind":"test"`, `"jvmTarget":"17"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}
