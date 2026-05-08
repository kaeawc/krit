package scan

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
)

// pickEditorConfigScanDir returns the directory used as the .editorconfig
// search root: the first scan-path arg, or "." when none are present.
// Pulled out as a pure function so the (tiny) edge case of "no positional
// args" is unit-testable.
func pickEditorConfigScanDir(scanArgs []string) string {
	if len(scanArgs) == 0 {
		return "."
	}
	return scanArgs[0]
}

// applyEditorConfigOverrides loads .editorconfig from the resolved scan
// directory, merges its values into cfg, and re-applies the config to
// the rule registry. Editorconfig takes precedence over YAML config —
// matching ktfmt's --style behavior. No-op when enabled is false.
func applyEditorConfigOverrides(cfg *config.Config, enabled bool, scanArgs []string) {
	if !enabled {
		return
	}
	ec := config.LoadEditorConfig(pickEditorConfigScanDir(scanArgs))
	ec.ApplyToConfig(cfg)
	rules.ApplyConfig(cfg)
}
