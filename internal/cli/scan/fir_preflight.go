package scan

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// PreflightFIR checks the requirements shared by direct and delegated CLI
// scans. Callers choose whether an error is fatal or a session notice.
func PreflightFIR(ctx context.Context, paths []string, cfg *config.Config, explicitModel string, noModel bool, warnings io.Writer) ([]string, error) {
	java, err := oracle.JavaPath()
	if err != nil {
		return nil, fmt.Errorf("FIR requires Java 21 or newer: %w; install a JDK or set JAVA_HOME, or pass --no-fir to scan with Go only", err)
	}
	if major := oracle.PreflightJavaMajorVersion(java); major < 21 {
		return nil, fmt.Errorf("FIR requires Java 21 or newer (found version %d at %s); install a JDK or set JAVA_HOME, or pass --no-fir to scan with Go only", major, java)
	}
	if _, err := oracle.EnsureBackendJar(ctx, oracle.BackendFIR, paths, false); err != nil {
		return nil, fmt.Errorf("FIR requires krit-fir.jar: %w; set KRIT_FIR_JAR to a local jar or pass --no-fir to scan with Go only", err)
	}
	modelDirs := make([]string, 0)
	if explicitModel != "" {
		modelDirs = append(modelDirs, explicitModel)
	} else if !noModel {
		for _, path := range paths {
			if dir := gradlemodel.Discover(path); dir != "" {
				modelDirs = append(modelDirs, dir)
			}
		}
	}
	if len(modelDirs) == 0 && (cfg == nil || !cfg.HasTopLevelOption("oracle", "classpath")) && len(dedupePreservingOrder(splitEnvClasspath())) == 0 {
		return nil, fmt.Errorf("FIR requires a declared classpath source: run Gradle's kritExportModel task, set oracle.classpath in krit.yml (including [] for stdlib-only projects), or set CLASSPATH; see docs/configuration.md for Maven and Bazel recipes; or pass --no-fir")
	}
	model, err := loadGradleClasspath(paths, explicitModel, noModel, false, warnings)
	if err != nil {
		return nil, fmt.Errorf("FIR Gradle model cannot be read: %w; run kritExportModel again or set oracle.classpath in krit.yml, or pass --no-fir", err)
	}
	for _, dir := range modelDirs {
		warnStaleGradleModel(dir, paths, warnings)
	}
	return model, nil
}

func warnStaleGradleModel(dir string, paths []string, out io.Writer) {
	model, _, err := gradlemodel.Load(dir)
	if err != nil {
		return
	}
	_, missing := model.Classpath()
	if missing > 0 {
		fmt.Fprintf(out, "warning: gradle model %s is stale: %d classpath jars missing; rerun kritExportModel\n", dir, missing)
	}
	newest := newestModelTime(dir)
	root := filepath.Dir(filepath.Dir(dir))
	if len(paths) > 0 && explicitRoot(paths[0]) != "" && newest.IsZero() {
		root = explicitRoot(paths[0])
	}
	dirs := []string{root}
	for _, project := range model.Projects {
		dirs = append(dirs, project.Dir)
	}
	seen := map[string]bool{}
	for _, projectDir := range dirs {
		projectDir = filepath.Clean(projectDir)
		if seen[projectDir] {
			continue
		}
		seen[projectDir] = true
		for _, name := range []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"} {
			path := filepath.Join(projectDir, name)
			if info, err := os.Stat(path); err == nil && info.ModTime().After(newest) {
				fmt.Fprintf(out, "warning: gradle model %s is stale: %s is newer; rerun kritExportModel\n", dir, path)
				return
			}
		}
	}
}

func newestModelTime(dir string) time.Time {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}
	}
	var newest time.Time
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

func explicitRoot(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}
