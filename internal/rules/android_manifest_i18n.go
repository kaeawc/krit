package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

var localeConfigResourceRef = regexp.MustCompile(`^@(?:[\w.]+:)?xml/([A-Za-z0-9_]+)$`)

// LocaleConfigMissingRule detects manifests that declare android:localeConfig
// but do not ship the referenced XML resource alongside the main manifest.
type LocaleConfigMissingRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest i18n rule. Detection flags missing locale configuration
// and translatable strings via attribute presence checks. Classified per
// roadmap/17.
func (r *LocaleConfigMissingRule) Confidence() float64 { return 0.75 }

func (r *LocaleConfigMissingRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil || m.Application.LocaleConfig == "" {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}

	resourceName, ok := localeConfigResourceName(m.Application.LocaleConfig)
	if !ok {
		return
	}

	resourcePath := filepath.Join(filepath.Dir(m.Path), "res", "xml", resourceName+".xml")
	if _, err := os.Stat(resourcePath); err == nil {
		return
	}

	f := manifestFinding(m.Path, m.Application.Line, r.BaseRule,
		fmt.Sprintf("<application> declares `android:localeConfig=\"%s\"` but `res/xml/%s.xml` is missing.",
			m.Application.LocaleConfig, resourceName))
	ctx.Emit(f)
	return
}

func localeConfigResourceName(ref string) (string, bool) {
	matches := localeConfigResourceRef.FindStringSubmatch(strings.TrimSpace(ref))
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}
