package scan

import (
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestPickEditorConfigScanDir(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no args defaults to dot", nil, "."},
		{"empty slice defaults to dot", []string{}, "."},
		{"single path returned verbatim", []string{"src/main"}, "src/main"},
		{"first path wins when multiple", []string{"a", "b", "c"}, "a"},
		{"empty-string first arg is preserved", []string{""}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickEditorConfigScanDir(tc.args)
			if got != tc.want {
				t.Fatalf("pickEditorConfigScanDir(%v) = %q; want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestApplyEditorConfigOverridesNoOpWhenDisabled(t *testing.T) {
	// When the flag is off, applyEditorConfigOverrides must not touch cfg
	// or call into rules.ApplyConfig. Pass a non-existent scan dir so any
	// stray LoadEditorConfig call would have no .editorconfig to find,
	// which combined with the no-op guarantee means the test is hermetic.
	cfg := config.NewConfig()
	applyEditorConfigOverrides(cfg, false, []string{"/nonexistent-dir-for-editor-config-test"})
	// Surviving this call without panicking is the assertion. cfg stays
	// the freshly-created default since LoadEditorConfig was never invoked.
}
