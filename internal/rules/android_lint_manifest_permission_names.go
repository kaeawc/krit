package rules

import (
	"math"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
)

// ReservedSystemPermissionRule mirrors Lint's ReservedSystemPermission: a <permission> that redeclares a platform
// permission or starts with the reserved `android.` prefix.
type ReservedSystemPermissionRule struct {
	ManifestBase
	AndroidRule
}

// PermissionNamingConventionRule mirrors Lint's PermissionNamingConvention: a custom permission whose name is not
// `<package>.permission.UPPER_SNAKE_CASE`.
type PermissionNamingConventionRule struct {
	ManifestBase
	AndroidRule
}

// KnownPermissionErrorRule mirrors Lint's KnownPermissionError: a permission attribute set to `true` or `false`.
type KnownPermissionErrorRule struct {
	ManifestBase
	AndroidRule
}

// SystemPermissionTypoRule mirrors Lint's SystemPermissionTypo: a requested permission that looks like a platform or
// system permission but is not one.
type SystemPermissionTypoRule struct {
	ManifestBase
	AndroidRule
}

// CustomPermissionTypoRule mirrors Lint's CustomPermissionTypo: a requested permission that looks like a permission
// declared in the same manifest but is not one.
type CustomPermissionTypoRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *ReservedSystemPermissionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *PermissionNamingConventionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *KnownPermissionErrorRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium base confidence: the typo match is an edit-distance heuristic.
func (r *SystemPermissionTypoRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a medium base confidence: the typo match is an edit-distance heuristic.
func (r *CustomPermissionTypoRule) Confidence() float64 { return api.ConfidenceMedium }

// permissionTables holds the permission name lists, sorted for binary search.
type permissionTables struct {
	platform []string
	system   []string
	// all is the system list followed by the platform list without repeats, in Lint's iteration order.
	all []string
}

var permissionNameTables = sync.OnceValue(func() permissionTables {
	t := permissionTables{
		platform: strings.Fields(androidPlatformPermissionNames),
		system:   strings.Fields(androidSystemPermissionNames),
	}
	seen := make(map[string]bool, len(t.platform)+len(t.system))
	for _, list := range [][]string{t.system, t.platform} {
		for _, name := range list {
			if !seen[name] {
				seen[name] = true
				t.all = append(t.all, name)
			}
		}
	}
	return t
})

func isSystemPermissionName(name string) bool {
	_, found := slices.BinarySearch(permissionNameTables().system, name)
	return found
}

func isPlatformPermissionName(name string) bool {
	_, found := slices.BinarySearch(permissionNameTables().platform, name)
	return found
}

// isStandardPermissionName reports whether Lint knows the permission as a system or SDK permission.
func isStandardPermissionName(name string) bool {
	return isSystemPermissionName(name) || isPlatformPermissionName(name)
}

// lintEditDistance is Lint's Lint.editDistance: the Levenshtein distance, or MaxInt32 when the lengths differ by more
// than maxDistance.
func lintEditDistance(s, t string, maxDistance int) int {
	if s == t {
		return 0
	}
	if diff := utf8.RuneCountInString(s) - utf8.RuneCountInString(t); diff > maxDistance || -diff > maxDistance {
		return math.MaxInt32
	}
	return nscEditDistance(s, t)
}

// substringAfterLastDot is Kotlin's substringAfterLast('.'): the whole string when it has no dot.
func substringAfterLastDot(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// platformPermissionPackageUnlikely reports whether the permission's package is too far from `android.permission`
// for it to be a mistyped platform permission.
func platformPermissionPackageUnlikely(name string) bool {
	prefix := name
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		prefix = name[:i]
	}
	prefix = strings.ToLower(prefix)
	switch prefix {
	case "android.permission", "android", "android.manifest.permission":
		return false
	}
	if strings.HasPrefix(prefix, "com.android.") {
		return false
	}
	for _, c := range name {
		if c != '.' && c != '_' && c != '$' && !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return false
		}
	}
	return lintEditDistance(prefix, "android.permission", 3) >= 3
}

// permissionTypoSanitized lowercases the name after removing characters a permission name cannot contain.
func permissionTypoSanitized(name string) string {
	var b strings.Builder
	for _, c := range name {
		if c == '.' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			b.WriteRune(c)
		}
	}
	return strings.ToLower(b.String())
}

// findAlmostPlatformPermission returns the system or SDK permission the name probably meant, or "".
func findAlmostPlatformPermission(required string) string {
	if isStandardPermissionName(required) || platformPermissionPackageUnlikely(required) {
		return ""
	}
	all := permissionNameTables().all
	requiredSuffix := substringAfterLastDot(required)
	for _, candidate := range all {
		if candidateSuffix := substringAfterLastDot(candidate); len(candidateSuffix) == len(requiredSuffix) &&
			strings.EqualFold(requiredSuffix, candidateSuffix) {
			return candidate
		}
		if strings.EqualFold(candidate, required) {
			return candidate
		}
	}
	cleaned := permissionTypoSanitized(required)
	cleanedSuffix := substringAfterLastDot(cleaned)
	best := ""
	maxDistance := 4
	for _, candidate := range all {
		lowered := strings.ToLower(candidate)
		if strings.Contains(cleaned, lowered) {
			return candidate
		}
		if d := lintEditDistance(cleanedSuffix, substringAfterLastDot(lowered), maxDistance); d < maxDistance {
			maxDistance = d
			best = candidate
		}
	}
	return best
}

// findAlmostCustomPermission returns the declared custom permission the name probably meant, or "".
func findAlmostCustomPermission(required string, custom []string) string {
	if slices.Contains(custom, required) {
		return ""
	}
	for _, candidate := range custom {
		if d := lintEditDistance(required, candidate, 4); d >= 1 && d < 4 {
			return candidate
		}
	}
	return ""
}

// followsCustomPermissionNamingConvention reports whether the name is `<package>...permission.UPPER_SNAKE_CASE`.
func followsCustomPermissionNamingConvention(pkg, name string) bool {
	if pkg == "" {
		return true
	}
	prefix, suffix := "", name
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		prefix, suffix = name[:i], name[i+1:]
	}
	if !strings.HasPrefix(prefix, pkg) || !strings.HasSuffix(prefix, "permission") || suffix == "" {
		return false
	}
	for _, c := range suffix {
		if c != '_' && (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

// permissionUsageTags are the elements whose android:permission attribute requires a permission.
var permissionUsageTags = map[string]bool{
	"application": true, "activity": true, "activity-alias": true, "receiver": true, "service": true, "provider": true,
}

// permissionUsageAttr returns the attribute that requires a permission on the element, if any.
func permissionUsageAttr(n *manifest.Node) (manifest.Attr, bool) {
	switch {
	case n.Tag == "uses-permission":
		return n.Attr("android:name")
	case permissionUsageTags[n.Tag]:
		return n.Attr("android:permission")
	}
	return manifest.Attr{}, false
}

// forEachPermissionUsage calls fn for every attribute that requires a permission, in document order.
func forEachPermissionUsage(root *manifest.Node, fn func(a manifest.Attr)) {
	manifestWalk(root, func(n, _ *manifest.Node) {
		if a, ok := permissionUsageAttr(n); ok {
			fn(a)
		}
	})
}

// forEachPermissionDefinition calls fn for the name of every <permission> element.
func forEachPermissionDefinition(root *manifest.Node, fn func(a manifest.Attr)) {
	manifestWalk(root, func(n, _ *manifest.Node) {
		if n.Tag != "permission" {
			return
		}
		if a, ok := n.Attr("android:name"); ok {
			fn(a)
		}
	})
}

func (r *ReservedSystemPermissionRule) check(ctx *api.Context) {
	forEachPermissionDefinition(ctx.Manifest.Root, func(a manifest.Attr) {
		switch {
		case isStandardPermissionName(a.Value):
			emitManifestAtValue(ctx, a, "`"+a.Value+"` is a reserved permission")
		case strings.HasPrefix(a.Value, "android."):
			emitManifestAtValue(ctx, a, "`"+a.Value+"` is using the reserved system prefix `android.`")
		}
	})
}

func (r *PermissionNamingConventionRule) check(ctx *api.Context) {
	forEachPermissionDefinition(ctx.Manifest.Root, func(a manifest.Attr) {
		if !followsCustomPermissionNamingConvention(ctx.Manifest.Package, a.Value) {
			emitManifestAtValue(ctx, a, "`"+a.Value+" does not follow recommended naming convention`")
		}
	})
}

func (r *KnownPermissionErrorRule) check(ctx *api.Context) {
	forEachPermissionUsage(ctx.Manifest.Root, func(a manifest.Attr) {
		if strings.EqualFold(a.Value, "true") || strings.EqualFold(a.Value, "false") {
			emitManifestAtValue(ctx, a, "`"+a.Value+"` is not a valid permission value")
		}
	})
}

func (r *SystemPermissionTypoRule) check(ctx *api.Context) {
	forEachPermissionUsage(ctx.Manifest.Root, func(a manifest.Attr) {
		if almost := findAlmostPlatformPermission(a.Value); almost != "" {
			emitManifestAtValue(ctx, a, "Did you mean `"+almost+"`?")
		}
	})
}

// customPermissionScope collects the permissions declared at the top level and every permission requirement, as
// Lint reads them from the merged manifest.
func customPermissionScope(root *manifest.Node) (custom []string, usages []manifest.Attr) {
	addUsage := func(n *manifest.Node, name string) {
		if a, ok := n.Attr(name); ok {
			usages = append(usages, a)
		}
	}
	for _, n := range root.Children {
		switch n.Tag {
		case "permission":
			if a, ok := n.Attr("android:name"); ok {
				custom = append(custom, a.Value)
			}
		case "uses-permission":
			addUsage(n, "android:name")
		case "application":
			addUsage(n, "android:permission")
			for _, c := range n.Children {
				if permissionUsageTags[c.Tag] {
					addUsage(c, "android:permission")
				}
			}
		}
	}
	return custom, usages
}

func (r *CustomPermissionTypoRule) check(ctx *api.Context) {
	root := ctx.Manifest.Root
	if root == nil {
		return
	}
	custom, usages := customPermissionScope(root)
	if len(custom) == 0 {
		return
	}
	for _, a := range usages {
		if almost := findAlmostCustomPermission(a.Value, custom); almost != "" {
			emitManifestAtValue(ctx, a, "Did you mean `"+almost+"`?")
		}
	}
}
