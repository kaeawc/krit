package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var localeConfigResourceRef = regexp.MustCompile(`^@(?:[\w.]+:)?xml/([A-Za-z0-9_]+)$`)

// LocaleConfigMissingRule detects manifests that declare android:localeConfig
// but do not ship the referenced XML resource alongside the main manifest.
type LocaleConfigMissingRule struct {
	ManifestBase
	AndroidRule
}

func (r *LocaleConfigMissingRule) check(ctx *api.Context) {
	m := ctx.Manifest
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

	f := baseFinding(m.Path, m.Application.Line, r.BaseRule,
		fmt.Sprintf("<application> declares `android:localeConfig=\"%s\"` but `res/xml/%s.xml` is missing.",
			m.Application.LocaleConfig, resourceName))
	ctx.Emit(f)
}

func localeConfigResourceName(ref string) (string, bool) {
	matches := localeConfigResourceRef.FindStringSubmatch(strings.TrimSpace(ref))
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}
