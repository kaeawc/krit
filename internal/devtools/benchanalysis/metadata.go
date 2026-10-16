//go:build unix

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Metadata is everything needed to reproduce a benchmark and judge its noise.
type Metadata struct {
	StartedAt     string            `json:"startedAt"`
	Corpus        string            `json:"corpus"`
	CorpusRev     string            `json:"corpusRevision"`
	CorpusDirty   bool              `json:"corpusDirty"`
	KritRev       string            `json:"kritRevision"`
	KritDirty     bool              `json:"kritDirty"`
	KritBinary    string            `json:"kritBinary"`
	KritSHA256    string            `json:"kritSha256"`
	KritVersion   string            `json:"kritVersion"`
	HelperJars    map[string]string `json:"helperJarSha256"`
	GoVersion     string            `json:"goVersion"`
	JavaVersion   string            `json:"javaVersion"`
	KotlinVersion string            `json:"kotlinVersion"`
	OS            string            `json:"os"`
	CPUs          int               `json:"cpus"`
	LoadAvg       string            `json:"loadAvgAtStart"`
	BuildModel    []string          `json:"buildModel"`
	Options       map[string]any    `json:"options"`
}

func collectMetadata(corpus, kritBin string, o options) Metadata {
	return Metadata{
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Corpus:        corpus,
		CorpusRev:     output("git", "-C", corpus, "rev-parse", "HEAD"),
		CorpusDirty:   output("git", "-C", corpus, "status", "--porcelain") != "",
		KritRev:       output("git", "rev-parse", "HEAD"),
		KritDirty:     output("git", "status", "--porcelain") != "",
		KritBinary:    kritBin,
		KritSHA256:    fileSHA256(kritBin),
		KritVersion:   output(kritBin, "--version"),
		HelperJars:    helperJars(),
		GoVersion:     runtime.Version(),
		JavaVersion:   firstLine(combined("java", "-version")),
		KotlinVersion: kotlinVersion("tools/krit-fir/build.gradle.kts"),
		OS:            output("uname", "-srm"),
		CPUs:          runtime.NumCPU(),
		LoadAvg:       loadAvg(),
		BuildModel:    buildModel(corpus),
		Options: map[string]any{
			"modes": o.modes, "states": o.states, "runs": o.runs, "editFile": o.editFile,
			"extraArgs": o.extraArgs, "isolate": o.isolate, "verify": o.verify,
		},
	}
}

// helperJars hashes every JVM helper jar krit could pick up. The jar a run
// actually used is in that run's oracleBackend attributes.
func helperJars() map[string]string {
	out := map[string]string{}
	var candidates []string
	for _, env := range []string{"KRIT_FIR_JAR", "KRIT_TYPES_JAR"} {
		if v := os.Getenv(env); v != "" {
			candidates = append(candidates, v)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, pattern := range []string{".krit/jars/*.jar", ".krit/jars/dev/*/*.jar"} {
			matches, _ := filepath.Glob(filepath.Join(home, pattern))
			candidates = append(candidates, matches...)
		}
	}
	for _, tool := range []string{"krit-fir", "krit-types"} {
		candidates = append(candidates, filepath.Join("tools", tool, "build", "libs", tool+".jar"))
	}
	for _, path := range candidates {
		if sum := fileSHA256(path); sum != "" {
			out[path] = sum
		}
	}
	return out
}

// buildModel lists the build files that determine how krit models the
// corpus's modules and classpath.
func buildModel(corpus string) []string {
	var found []string
	for _, name := range []string{"settings.gradle.kts", "settings.gradle", "build.gradle.kts", "build.gradle",
		"gradle/libs.versions.toml", "krit.yml", ".krit.yml", "pom.xml"} {
		if _, err := os.Stat(filepath.Join(corpus, name)); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// kotlinVersion reads the embedded compiler version the helper jars are
// built against (`val kotlinVersion = "x.y.z"` in the build script).
func kotlinVersion(buildScript string) string {
	data, err := os.ReadFile(buildScript)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "val kotlinVersion ="); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func output(name string, args ...string) string {
	out, err := exec.CommandContext(context.Background(), name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func combined(name string, args ...string) string {
	out, _ := exec.CommandContext(context.Background(), name, args...).CombinedOutput()
	return string(out)
}

// firstLine skips JVM banner noise such as "Picked up JAVA_TOOL_OPTIONS".
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Picked up ") {
			return line
		}
	}
	return ""
}
