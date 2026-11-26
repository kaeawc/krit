package modshard

// settings.go — split the FIR check into one session per group of source sets
// that share their Kotlin compiler settings (#1577).
//
// The whole-repository check compiles every source set in one session with
// merged settings: opt-ins are unioned, flags intersected, and a plugin option
// or a language level that differs between source sets is dropped. Each group
// of source sets with identical settings whose settings differ from the merged
// ones is compiled again as one module, with its own settings and its
// dependencies as built jars or context sources, and its verdicts replace the
// merged ones.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// SettingsGroupingKey names what a settings group shares, for the verbose log.
const SettingsGroupingKey = "compiler arguments (language/API version, opt-ins, progressive, flags), compiler plugin jars and options, JVM target"

// settingsGroup is the scanned source sets that share one compile configuration.
type settingsGroup struct {
	units []Unit
	// args, plugins and options are the group's compiler arguments and
	// compiler plugins, the same for every unit.
	args, plugins, options []string
	jvmTarget, platform    string
}

func (g *settingsGroup) label() string {
	ids := make([]string, len(g.units))
	for i, u := range g.units {
		ids[i] = u.Spec.ID
	}
	settings := strings.Join(g.args, " ")
	if len(g.plugins) > 0 {
		names := make([]string, len(g.plugins))
		for i, p := range g.plugins {
			names[i] = filepath.Base(p)
		}
		settings += " plugins=" + strings.Join(names, ",")
	}
	if len(g.options) > 0 {
		settings += " " + strings.Join(g.options, " ")
	}
	if settings = strings.TrimSpace(settings); settings == "" {
		settings = "(compiler defaults)"
	}
	return fmt.Sprintf("%s [%s]", summarizeIDs(ids), settings)
}

func summarizeIDs(ids []string) string {
	if len(ids) <= 4 {
		return strings.Join(ids, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(ids[:4], ", "), len(ids)-4)
}

// settingsKey is the grouping key of a unit: everything that makes two source
// sets compile differently besides their sources and classpath.
func settingsKey(spec firchecks.ModuleSpec) string {
	parts := []string{spec.Platform, spec.JVMTarget, "|args"}
	parts = append(parts, spec.CompilerArgs...)
	parts = append(parts, "|plugins")
	parts = append(parts, sortedCopy(spec.CompilerPluginClasspath)...)
	parts = append(parts, "|options")
	parts = append(parts, spec.CompilerPluginOptions...)
	return strings.Join(parts, "\x00")
}

// appliedKey is the part of the settings the merged session applies, so a
// group whose key equals the merged session's needs no session of its own.
// The embeddable compiler jar ModuleSpecsFromModel adds for embeddable
// plugins is not a setting of its own and is ignored.
func appliedKey(args, plugins, options []string) string {
	var jars []string
	for _, jar := range plugins {
		if !strings.HasPrefix(filepath.Base(jar), "kotlin-compiler-embeddable-") {
			jars = append(jars, jar)
		}
	}
	return strings.Join(sortedCopy(args), "\x00") + "\x01" + strings.Join(sortedCopy(jars), "\x00") + "\x01" + strings.Join(sortedCopy(options), "\x00")
}

func sortedCopy(in []string) []string {
	out := slices.Clone(in)
	sort.Strings(out)
	return out
}

// groupBySettings groups units with identical settings, in order of first
// appearance. Multiplatform units (HMPP fragments) and units whose dependency
// context was dropped are left out: they keep the merged verdicts.
func groupBySettings(units []Unit) (groups []*settingsGroup, left []string) {
	byKey := map[string]*settingsGroup{}
	for _, u := range units {
		if len(u.Spec.Fragments) > 0 || u.ContextDropped {
			left = append(left, u.Spec.ID)
			continue
		}
		key := settingsKey(u.Spec)
		g := byKey[key]
		if g == nil {
			g = &settingsGroup{args: u.Spec.CompilerArgs, plugins: u.Spec.CompilerPluginClasspath,
				options: u.Spec.CompilerPluginOptions, jvmTarget: u.Spec.JVMTarget, platform: u.Spec.Platform}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.units = append(g.units, u)
	}
	return groups, left
}

// groupUnit merges a group's units into one module compiled with their shared
// settings: their source roots together, the union of their classpaths and
// friends, and, as context, the sources of dependencies outside the group
// that have no built jar.
func groupUnit(index int, g *settingsGroup) Unit {
	first := g.units[0].Spec
	spec := firchecks.ModuleSpec{
		ID:                      fmt.Sprintf(":krit-settings-%d:group", index),
		Platform:                first.Platform,
		Kind:                    "main",
		JVMTarget:               g.jvmTarget,
		CompilerArgs:            slices.Clone(g.args),
		CompilerPluginClasspath: slices.Clone(g.plugins),
		CompilerPluginOptions:   slices.Clone(g.options),
		DependsOn:               []string{},
	}
	if spec.CompilerArgs == nil {
		spec.CompilerArgs = []string{}
	}
	allMain := true
	own := map[string]bool{}
	for _, u := range g.units {
		if u.Spec.Kind != "main" {
			allMain = false
		}
		for _, root := range append(append([]string{}, u.Spec.SourceRoots...), u.Spec.GeneratedSourceRoots...) {
			own[filepath.Clean(root)] = true
		}
	}
	if !allMain && len(g.units) == 1 {
		spec.Kind = first.Kind
	}
	appendNew := func(dst []string, src []string, skip map[string]bool) []string {
		have := map[string]bool{}
		for _, e := range dst {
			have[e] = true
		}
		for _, e := range src {
			if e != "" && !have[e] && !skip[filepath.Clean(e)] {
				have[e] = true
				dst = append(dst, e)
			}
		}
		return dst
	}
	var files int
	var sources []string
	for _, u := range g.units {
		spec.SourceRoots = appendNew(spec.SourceRoots, u.Spec.SourceRoots, nil)
		spec.GeneratedSourceRoots = appendNew(spec.GeneratedSourceRoots, u.Spec.GeneratedSourceRoots, nil)
		spec.Classpath = appendNew(spec.Classpath, u.Spec.Classpath, nil)
		spec.Friends = appendNew(spec.Friends, u.Spec.Friends, nil)
		spec.ContextSourceRoots = appendNew(spec.ContextSourceRoots, u.Spec.ContextSourceRoots, own)
		files += u.Files
		sources = append(sources, u.Sources...)
	}
	for _, list := range []*[]string{&spec.SourceRoots, &spec.GeneratedSourceRoots, &spec.Classpath, &spec.Friends} {
		if *list == nil {
			*list = []string{}
		}
	}
	sort.Strings(sources)
	return Unit{Spec: spec, Files: files, Sources: slices.Compact(sources)}
}

// uniformSettings reports whether every source set of the model has the same
// settings key, so no scanned subset can need a session of its own.
func uniformSettings(model *gradlemodel.Model) bool {
	first := ""
	for i, spec := range firchecks.ModuleSpecsFromModel(model) {
		key := settingsKey(spec)
		if i == 0 {
			first = key
		} else if key != first {
			return false
		}
	}
	return true
}

// SplitResult describes a settings split for the verbose log and perf entry.
type SplitResult struct {
	// Sessions is the number of FIR sessions the check used: the merged
	// whole-repository session plus one per group compiled on its own.
	Sessions int
	// Groups is the number of distinct settings groups among the scanned source sets.
	Groups int
	// Labels describes each group compiled on its own.
	Labels []string
}

// SettingsSplitter returns the FIR pass's per-settings-group recheck for a scan
// of scanPaths, or nil when KRIT_MODULE_SHARDS=merged turns it off or the scan
// reads no Gradle model. Arguments are those of Isolator.
func SettingsSplitter(scanPaths []string, modelDir string, modelDisabled, noCache bool, logf func(string, ...any)) firchecks.SettingsSplitter {
	if MergedOnly() || len(scanPaths) == 0 || (modelDir == "" && modelDisabled) {
		return nil
	}
	scanPaths = append([]string(nil), scanPaths...)
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return func(ctx context.Context, jarPath string, _ []string, request []byte) ([]byte, []string, error) {
		if jarPath == "" {
			return nil, nil, nil
		}
		javaPath, err := oracle.JavaPath()
		if err != nil {
			logf("verbose: FIR settings: not split: %v\n", err)
			return nil, nil, nil
		}
		var cacheDir string
		if noCache {
			if cacheDir, err = os.MkdirTemp("", "krit-settings-groups-"); err != nil {
				return nil, nil, err
			}
			defer os.RemoveAll(cacheDir)
		} else {
			repoDir := oracle.FindRepoDir(scanPaths)
			if repoDir == "" {
				logf("verbose: FIR settings: not split: no project root above the scan paths for the cache\n")
				return nil, nil, nil
			}
			cacheDir = filepath.Join(fsutil.ProjectStateDir(repoDir), "module-facts", "settings-groups")
		}
		resp, rechecked, _, err := CheckSettingsGroups(ctx, Input{
			ScanPaths:    scanPaths,
			ModelDir:     modelDir,
			JarPath:      jarPath,
			JavaPath:     javaPath,
			CacheDir:     cacheDir,
			CheckRequest: request,
			Logf:         logf,
		})
		return resp, rechecked, err
	}
}

// CheckSettingsGroups runs the FIR check of in.CheckRequest once per group of
// scanned source sets whose settings differ from the merged session's (the
// request's kotlinArgs and compiler plugins), each group compiled as one
// module with its own settings. It returns the merged check response for the
// files those sessions checked and those files; both are empty when every
// scanned source set compiles like the merged session.
func CheckSettingsGroups(ctx context.Context, in Input) ([]byte, []string, SplitResult, error) {
	logf := in.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var split SplitResult
	if len(in.ScanPaths) == 0 || in.JarPath == "" || in.JavaPath == "" || in.CacheDir == "" {
		return nil, nil, split, nil
	}
	check := parseCheckContext(in.CheckRequest)
	if check == nil {
		return nil, nil, split, nil
	}
	model, repoRoot, reason := isolationModel(in)
	if model == nil {
		logf("verbose: FIR settings: not split: %s\n", reason)
		return nil, nil, split, nil
	}
	units, split := planSettingsSessions(in, model, repoRoot, check, logf)
	if len(units) == 0 {
		return nil, nil, split, nil
	}
	if err := os.MkdirAll(in.CacheDir, 0o755); err != nil {
		return nil, nil, split, err
	}
	keys, stale, keyOf, requested := keyUnits(in, units, newIdentities(), check)
	if len(stale) > 0 {
		if _, _, err := analyze(ctx, in, stale, keyOf); err != nil {
			return nil, nil, split, err
		}
	}
	done, doneKeys, rechecked := finishedUnits(in.CacheDir, units, keys, requested)
	if len(rechecked) == 0 {
		return nil, nil, split, nil
	}
	sort.Strings(rechecked)
	check.files = make(map[string]bool, len(rechecked))
	for _, f := range rechecked {
		check.files[f] = true
	}
	resp, err := mergeChecks(in, done, doneKeys, requested, check)
	if err != nil {
		return nil, nil, split, err
	}
	prune(in.CacheDir, keys)
	return resp, rechecked, split, nil
}

// planSettingsSessions returns the group units that need a session of their
// own, one per settings group that differs from the merged session, and logs
// the plan.
func planSettingsSessions(in Input, model *gradlemodel.Model, repoRoot string, check *checkContext, logf func(string, ...any)) ([]Unit, SplitResult) {
	split := SplitResult{Groups: 1, Sessions: 1}
	if uniformSettings(model) {
		// Plan walks every source root; most models need no split at all.
		logf("verbose: FIR settings: 1 session (every source set shares its compiler settings)\n")
		return nil, split
	}
	all, _ := Plan(model, repoRoot, in.ScanPaths)
	addUserClasspath(all, model, check.classpath)
	if len(all) == 0 {
		logf("verbose: FIR settings: not split: no Gradle source set under the scan paths %s\n", strings.Join(in.ScanPaths, ", "))
		return nil, split
	}
	groups, left := groupBySettings(all)
	split.Groups = len(groups)
	if len(groups) == 0 {
		logf("verbose: FIR settings: 1 session (every scanned source set is multiplatform or context-limited)\n")
		return nil, split
	}
	// Even one scanned group can differ from the merged session, whose
	// settings merge every source set of the model. Shared sources are found
	// across every group, including those the merged session already compiles
	// exactly, so a file one of those also owns keeps its merged verdict.
	merged := appliedKey(check.kotlinArgs, check.pluginClasspath, check.pluginOptions)
	var own []*settingsGroup
	for _, g := range withoutSharedSources(groups, logf) {
		if appliedKey(g.args, g.plugins, g.options) != merged {
			own = append(own, g)
		}
	}
	if len(left) > 0 {
		logf("verbose: FIR settings: %d multiplatform or context-limited source sets keep the merged session: %s\n", len(left), summarizeIDs(left))
	}
	if len(own) == 0 {
		logf("verbose: FIR settings: 1 session (%d settings groups, but the merged session applies every group's settings)\n", len(groups))
		return nil, split
	}
	units := make([]Unit, len(own))
	for i, g := range own {
		units[i] = groupUnit(i+1, g)
		split.Labels = append(split.Labels, g.label())
	}
	split.Sessions = 1 + len(units)
	logf("verbose: FIR settings: %d sessions for %d settings groups (grouping key: %s); merged session plus %d compiled on their own:\n", split.Sessions, len(groups), SettingsGroupingKey, len(units))
	for i, label := range split.Labels {
		logf("verbose: FIR settings: session %d: %s\n", i+2, label)
	}
	return units, split
}

// withoutSharedSources drops the groups that share a source file with another
// group: that file would get two verdicts. Their files keep the merged ones.
func withoutSharedSources(groups []*settingsGroup, logf func(string, ...any)) []*settingsGroup {
	owner := map[string]int{}
	shared := map[int]bool{}
	for i, g := range groups {
		for _, u := range g.units {
			for _, f := range u.Sources {
				path := filepath.Clean(f)
				if prev, ok := owner[path]; ok && prev != i {
					shared[prev], shared[i] = true, true
				}
				owner[path] = i
			}
		}
	}
	if len(shared) == 0 {
		return groups
	}
	var out []*settingsGroup
	for i, g := range groups {
		if shared[i] {
			logf("verbose: FIR settings: %s keeps the merged session (a source file belongs to another settings group too)\n", g.label())
			continue
		}
		out = append(out, g)
	}
	return out
}
