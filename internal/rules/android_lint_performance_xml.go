package rules

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's ManifestDetector WearableBindListener check and StringCasingDetector
// (DuplicateStrings).

func registerPerformanceXMLChecks() {
	wear := &WearableBindListenerRule{AndroidRule: lintAndroidRule("WearableBindListener",
		"Usage of Android Wear BIND_LISTENER is deprecated", ALCPerformance, ALSFatal, 6)}
	api.Register(&api.Rule{
		ID: wear.RuleName, Category: wear.RuleSetName, Description: wear.Description(), Sev: api.Severity(wear.Sev),
		Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), NeedsLibraryFacts: true,
		Confidence: wear.Confidence(), Implementation: wear, Check: wear.check,
	})
	dup := &DuplicateStringsRule{AndroidRule: lintAndroidRule("DuplicateStrings", "Duplicate Strings", ALCPerformance,
		ALSWarning, 2)}
	registerLintResourceRule(dup.AndroidRule, dup, dup.Confidence(), dup.check)
}

// ---------------------------------------------------------------------------
// WearableBindListener
// ---------------------------------------------------------------------------

// WearableBindListenerRule mirrors Lint's WearableBindListener: a service registered for the Android Wear
// BIND_LISTENER action, which wakes the app for every Wear event.
type WearableBindListenerRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports high confidence: the action is read from the manifest and the dependency from Gradle facts.
func (r *WearableBindListenerRule) Confidence() float64 { return api.ConfidenceHigh }

const wearBindListenerAction = "com.google.android.gms.wearable.BIND_LISTENER"

func (r *WearableBindListenerRule) check(ctx *api.Context) {
	if ctx.Manifest == nil || ctx.Manifest.Root == nil {
		return
	}
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	// Lint checks services only in Gradle projects, whose model supplies the dependency and the build SDK.
	if facts == nil || !facts.Profile.HasGradle {
		return
	}
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "service" {
			return
		}
		// Lint keeps the action of the last intent filter that declares it.
		var found *manifest.Attr
		for _, filter := range n.Children {
			if filter.Tag != "intent-filter" {
				continue
			}
			for _, action := range filter.Children {
				if action.Tag != "action" {
					continue
				}
				if a, ok := action.Attr("android:name"); ok && a.Value == wearBindListenerAction {
					a := a
					found = &a
					break
				}
			}
		}
		if found == nil {
			return
		}
		switch {
		case wearableGmsAtLeast82(facts.Profile):
			emitManifestAtAttr(ctx, *found, "The `com.google.android.gms.wearable.BIND_LISTENER` action is deprecated")
		case wearCompileSdk(facts.Profile) >= 24:
			// Lint names the newest installed play-services-wearable when the SDK's Maven repository has a newer
			// one; krit does not read the SDK, so it reports the generic upgrade message.
			emitManifestAtAttr(ctx, *found, "The `com.google.android.gms.wearable.BIND_LISTENER` action is deprecated. "+
				"Please upgrade to the latest version of play-services-wearable 8.2.0 or later")
		}
	})
}

// wearableGmsAtLeast82 reports whether the project depends on play-services-wearable 8.2.0 or later. A declared
// dependency whose version krit cannot read counts as recent.
func wearableGmsAtLeast82(profile librarymodel.ProjectProfile) bool {
	for _, d := range profile.Dependencies {
		if d.Group != "com.google.android.gms" || d.Name != "play-services-wearable" {
			continue
		}
		v := parseGradleVersion(d.Version)
		if !v.known {
			return true
		}
		if v.atLeast(8, 2, 0) {
			return true
		}
	}
	return false
}

func wearCompileSdk(profile librarymodel.ProjectProfile) int {
	if n, err := strconv.Atoi(strings.TrimSpace(profile.Android.CompileSDK.Version)); err == nil {
		return n
	}
	return 0
}

// ---------------------------------------------------------------------------
// DuplicateStrings
// ---------------------------------------------------------------------------

// DuplicateStringsRule mirrors Lint's DuplicateStrings: string resources of one locale whose values are equal
// ignoring case.
type DuplicateStringsRule struct {
	ResourceBase
	AndroidRule
}

// Confidence reports high confidence: every values folder of the project is compared.
func (r *DuplicateStringsRule) Confidence() float64 { return api.ConfidenceHigh }

// AndroidDependencies declares that the rule reads values resources.
func (r *DuplicateStringsRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepValuesStrings
}

func (r *DuplicateStringsRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	if idx == nil || len(idx.StringDefinitions) == 0 {
		return
	}
	defs := append([]android.StringDefinition(nil), idx.StringDefinitions...)
	// Lint visits resource folders and their files in name order, and elements in document order.
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].FilePath != defs[j].FilePath {
			return resourceVisitKey(defs[i].FilePath) < resourceVisitKey(defs[j].FilePath)
		}
		return defs[i].ElementLine < defs[j].ElementLine || defs[i].ElementLine == defs[j].ElementLine && defs[i].Line < defs[j].Line
	})
	type group struct {
		defs []android.StringDefinition
		text []string
	}
	groups := map[string]*group{}
	var order []string
	for _, d := range defs {
		if d.Text == "" || d.HasElements || d.Translatable == "false" {
			continue
		}
		text := lintStripQuotes(d.Text)
		key := duplicateStringsLocale(d.FilePath) + "\x00" + strings.ToLower(text)
		g := groups[key]
		if g == nil {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.defs = append(g.defs, d)
		g.text = append(g.text, text)
	}
	for _, key := range order {
		g := groups[key]
		if len(g.defs) < 2 {
			continue
		}
		caseVaries := false
		names := make([]string, 0, len(g.defs))
		for i, d := range g.defs {
			names = append(names, "`"+d.Name+"`")
			if i > 0 && g.text[i] != g.text[i-1] {
				caseVaries = true
			}
		}
		sort.Strings(names)
		msg := "Duplicate string value `" + g.text[len(g.text)-1] + "`, used in " + lintFormatConjunction(names)
		if caseVaries {
			msg += ". Use `android:inputType` or `android:capitalize` to treat these as the same and avoid string duplication."
		}
		first := g.defs[0]
		line, col := first.ElementLine, first.ElementCol
		if line == 0 {
			line, col = first.Line, 1
		}
		ctx.Emit(scanner.Finding{File: first.FilePath, Line: line, Col: col, Message: msg})
	}
}

// resourceVisitKey orders resource files as Lint visits them: by resource directory, then folder name (so values
// comes before values-night), then file name.
func resourceVisitKey(path string) string {
	path = filepath.ToSlash(path)
	dir, file := filepath.Split(path)
	dir = strings.TrimSuffix(dir, "/")
	root, folder := filepath.Split(dir)
	return root + "\x00" + folder + "\x00" + file
}

// duplicateStringsLocale is the locale key Lint groups a values folder's strings by: the language and region (or
// BCP 47 tag) of the folder, or "default" for folders without a language qualifier.
func duplicateStringsLocale(path string) string {
	folder := filepath.Base(filepath.Dir(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/"))))
	if loc, ok := parseFolderLocale(folder); ok && loc.language != "" {
		return loc.language + "-" + loc.region
	}
	return "default"
}

// lintFormatConjunction joins items like Lint's formatList(useConjunction = true): "a, b and c".
func lintFormatConjunction(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func (r *WearableBindListenerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearableBindListener", RuleSet: "android-lint", DefaultActive: true}
}

// DuplicateStrings is off by default in Lint: deduplicating strings is a size trade-off a project opts into.
func (r *DuplicateStringsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DuplicateStrings", RuleSet: "android-lint", DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated}
}
