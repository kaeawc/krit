package modshard

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// ShardsEnv selects how the oracle and the FIR check compile: unset compiles the scanned source sets
// together and rechecks only projects with a duplicate declaration between them, "merged" never
// rechecks, "isolate" compiles every scanned source set on its own, and a number or "auto" shards
// the oracle by module.
const ShardsEnv = "KRIT_MODULE_SHARDS"

// MergedOnly reports whether KRIT_MODULE_SHARDS=merged turns source-set isolation off.
func MergedOnly() bool { return os.Getenv(ShardsEnv) == "merged" }

// Isolator returns the FIR pass's source-set isolator for a scan of scanPaths, or nil when
// KRIT_MODULE_SHARDS=merged turns it off or the scan reads no Gradle model. modelDir is the scan's
// explicit model directory ("" discovers one from the scan paths unless modelDisabled). Rechecked units
// are cached by content under the project state directory, or in a directory removed after the recheck
// when noCache is set.
func Isolator(scanPaths []string, modelDir string, modelDisabled, noCache bool, logf func(string, ...any)) firchecks.SourceSetIsolator {
	if MergedOnly() || len(scanPaths) == 0 || (modelDir == "" && modelDisabled) {
		return nil
	}
	scanPaths = append([]string(nil), scanPaths...)
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return func(ctx context.Context, jarPath string, gated []string, request []byte) ([]byte, []string, error) {
		if jarPath == "" {
			return nil, nil, nil
		}
		javaPath, err := oracle.JavaPath()
		if err != nil {
			logf("verbose: FIR source sets: not rechecking gated files: %v\n", err)
			return nil, nil, nil
		}
		var cacheDir string
		if noCache {
			if cacheDir, err = os.MkdirTemp("", "krit-source-sets-"); err != nil {
				return nil, nil, err
			}
			defer os.RemoveAll(cacheDir)
		} else {
			repoDir := oracle.FindRepoDir(scanPaths)
			if repoDir == "" {
				logf("verbose: FIR source sets: not rechecking gated files: no project root above the scan paths for the cache\n")
				return nil, nil, nil
			}
			cacheDir = filepath.Join(fsutil.ProjectStateDir(repoDir), "module-facts", "source-sets")
		}
		return CheckIsolated(ctx, Input{
			ScanPaths:    scanPaths,
			ModelDir:     modelDir,
			JarPath:      jarPath,
			JavaPath:     javaPath,
			CacheDir:     cacheDir,
			CheckRequest: request,
			Logf:         logf,
		}, gated)
	}
}

// CheckIsolated runs the FIR check of in.CheckRequest over the projects that own a gated file, with
// each of their scanned source sets compiled on its own: a test or androidTest set sees its project's
// main set but never a sibling, and no set sees another project's sets except as dependency context.
// A project qualifies when each of its scanned sets has its full dependency context. Of a qualifying
// project, only the sets that hold a requested file are compiled: the caller requests just the files
// whose merged verdict it may replace (#1661). Gated files in variant directories no exported set
// lists are rechecked by variantUnits. It returns the merged check response for the rechecked files
// and those files (as the request spells them); both are empty when no project qualified.
func CheckIsolated(ctx context.Context, in Input, gated []string) ([]byte, []string, error) {
	logf := in.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if len(gated) == 0 || len(in.ScanPaths) == 0 || in.JarPath == "" || in.JavaPath == "" || in.CacheDir == "" {
		return nil, nil, nil
	}
	check := parseCheckContext(in.CheckRequest)
	if check == nil {
		return nil, nil, nil
	}
	model, repoRoot, reason := isolationModel(in)
	if model == nil {
		logf("verbose: FIR source sets: not rechecking %d gated files: %s\n", len(gated), reason)
		return nil, nil, nil
	}
	all, _ := Plan(model, repoRoot, in.ScanPaths)
	if len(all) == 0 {
		logf("verbose: FIR source sets: not rechecking %d gated files: no Gradle source set under the scan paths %s\n", len(gated), strings.Join(in.ScanPaths, ", "))
		return nil, nil, nil
	}
	units, skipped, unowned := isolationUnits(all, gated)
	units = holdingRequested(units, check)
	variants, beyond := variantUnits(model, all, skipped, unowned)
	units = append(units, variants...)
	addUserClasspath(units, model, check.classpath)
	for _, project := range skipped {
		logf("verbose: FIR source sets: %s keeps its merged verdicts (dependency sources exceed the context limit, or a source file belongs to another project too)\n", project)
	}
	if len(beyond) > 0 {
		logf("verbose: FIR source sets: %d variant source sets beyond the limit of %d keep their merged verdicts: %s\n", len(beyond), maxVariantUnits, strings.Join(beyond, ", "))
	}
	if len(units) == 0 {
		return nil, nil, nil
	}
	logf("verbose: FIR source sets: rechecking %d source sets of %d projects (%d variant source sets outside the Gradle model)\n", len(units), countProjects(units), len(variants))
	if err := os.MkdirAll(in.CacheDir, 0o755); err != nil {
		return nil, nil, err
	}
	keys, stale, keyOf, requested := keyUnits(in, units, newIdentities(), check)
	if len(stale) > 0 {
		if _, _, err := analyze(ctx, in, stale, keyOf); err != nil {
			return nil, nil, err
		}
	}
	done, doneKeys, rechecked := finishedUnits(in.CacheDir, units, keys, requested)
	if len(rechecked) == 0 {
		return nil, nil, nil
	}
	sort.Strings(rechecked)
	check.files = make(map[string]bool, len(rechecked))
	for _, f := range rechecked {
		check.files[f] = true
	}
	resp, err := mergeChecks(in, done, doneKeys, requested, check)
	if err != nil {
		return nil, nil, err
	}
	prune(in.CacheDir, keys)
	return resp, rechecked, nil
}

// isolationUnits returns the units of every project that owns a gated file, the owning projects
// left out because a unit lost dependency context or shares a source file with another project, and
// the gated files no unit owns. A project with one scanned set qualifies too: the
// whole-repository compile also merges it with other projects' sets, which can declare the same class.
func isolationUnits(units []Unit, gated []string) (kept []Unit, skipped, unowned []string) {
	owner := map[string]string{}
	shared := map[string]bool{}
	byProject := map[string][]Unit{}
	for _, u := range units {
		project := projectOf(u.Spec.ID)
		byProject[project] = append(byProject[project], u)
		for _, f := range u.Sources {
			path := filepath.Clean(f)
			if prev, ok := owner[path]; ok && prev != project {
				shared[prev], shared[project] = true, true
			}
			owner[path] = project
		}
	}
	wanted := map[string]bool{}
	for _, g := range gated {
		path := filepath.Clean(g)
		project, ok := owner[path]
		if !ok {
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				project, ok = owner[resolved]
			}
		}
		if ok {
			wanted[project] = true
		} else {
			unowned = append(unowned, path)
		}
	}
	projects := make([]string, 0, len(wanted))
	for p := range wanted {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	for _, p := range projects {
		// A source file another project also lists would be answered by both.
		if shared[p] || incompleteContext(byProject[p]) != "" {
			skipped = append(skipped, p)
			continue
		}
		kept = append(kept, byProject[p]...)
	}
	return kept, skipped, unowned
}

// holdingRequested returns the units that hold a requested file. A unit holding none would be
// compiled for nothing: the caller requests only the files whose merged verdict it may replace.
func holdingRequested(units []Unit, check *checkContext) []Unit {
	var out []Unit
	for _, u := range units {
		if len(check.requestedAmong(u.Sources)) > 0 {
			out = append(out, u)
		}
	}
	return out
}

// finishedUnits returns the units whose check finished, their keys, and the files they checked. A
// failed unit keeps the merged verdicts, and so does every unit of a project in which two units share
// a file (variant sets with a common root), which would otherwise be answered twice.
func finishedUnits(cacheDir string, units []Unit, keys []string, requested map[string][]string) (done []Unit, doneKeys, rechecked []string) {
	shared := projectsSharingFiles(units, requested)
	for i, u := range units {
		if shared[projectOf(u.Spec.ID)] {
			continue
		}
		if own := requested[u.Spec.ID]; len(own) > 0 && cached(cacheDir, keys[i], true) {
			done = append(done, u)
			doneKeys = append(doneKeys, keys[i])
			rechecked = append(rechecked, own...)
		}
	}
	return done, doneKeys, rechecked
}

// projectsSharingFiles returns the projects in which two units request the same file.
func projectsSharingFiles(units []Unit, requested map[string][]string) map[string]bool {
	owner := map[string]string{}
	shared := map[string]bool{}
	for _, u := range units {
		for _, f := range requested[u.Spec.ID] {
			if other, ok := owner[f]; ok && other != u.Spec.ID {
				shared[projectOf(u.Spec.ID)] = true
			}
			owner[f] = u.Spec.ID
		}
	}
	return shared
}

func countProjects(units []Unit) int {
	seen := map[string]bool{}
	for _, u := range units {
		seen[projectOf(u.Spec.ID)] = true
	}
	return len(seen)
}

// isolationModel loads the scan's Gradle model: in.ModelDir when set, else the one discovered from the
// first scan path, with its directories spelled like the scan paths. The repository root is where built
// dependency jars are looked up. reason says why there is no model.
func isolationModel(in Input) (model *gradlemodel.Model, repoRoot, reason string) {
	if in.ModelDir == "" {
		return loadModel(in.ScanPaths)
	}
	model, _, err := gradlemodel.Load(in.ModelDir)
	if err != nil || model == nil {
		return nil, "", "Gradle model " + in.ModelDir + " not loadable"
	}
	alignModelPaths(model, in.ScanPaths)
	repoRoot = oracle.FindRepoDir(in.ScanPaths)
	if repoRoot == "" {
		repoRoot = filepath.Dir(filepath.Dir(in.ModelDir))
	}
	return model, repoRoot, ""
}

// addUserClasspath appends to every unit the request's classpath entries that no model source set
// lists: the ones the user configured (oracle.classpath, CLASSPATH), which the merged compile had too.
func addUserClasspath(units []Unit, model *gradlemodel.Model, requested []string) {
	fromModel := map[string]bool{}
	for _, p := range model.Projects {
		for _, s := range p.SourceSets {
			for _, list := range [][]string{s.BootClasspath, s.ClasspathEntries, s.GeneratedClasspath} {
				for _, entry := range list {
					fromModel[oracle.AbsolutePath(entry)] = true
				}
			}
		}
	}
	var extra []string
	for _, entry := range requested {
		if !fromModel[oracle.AbsolutePath(entry)] {
			extra = append(extra, entry)
		}
	}
	if len(extra) == 0 {
		return
	}
	for i := range units {
		have := map[string]bool{}
		for _, entry := range units[i].Spec.Classpath {
			have[oracle.AbsolutePath(entry)] = true
		}
		for _, entry := range extra {
			if !have[oracle.AbsolutePath(entry)] {
				have[oracle.AbsolutePath(entry)] = true
				units[i].Spec.Classpath = append(units[i].Spec.Classpath, entry)
			}
		}
	}
}
