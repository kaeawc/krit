package firchecks

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/gradlemodel"
)

// ModuleFragment describes one HMPP fragment and its refinement edges.
type ModuleFragment struct {
	Name        string   `json:"name"`
	SourceRoots []string `json:"sourceRoots"`
	Refines     []string `json:"refines"`
}

// ModuleSpec is one JVM/Android compilation. Classpath order is significant.
// Platform, Kind and JVMTarget are omitted when unset so krit-fir applies its
// defaults (jvm, main, and the session's JVM target).
type ModuleSpec struct {
	ID                   string           `json:"id"`
	Platform             string           `json:"platform,omitempty"`
	Kind                 string           `json:"kind,omitempty"`
	SourceRoots          []string         `json:"sourceRoots"`
	GeneratedSourceRoots []string         `json:"generatedSourceRoots"`
	Classpath            []string         `json:"classpath"`
	DependsOn            []string         `json:"dependsOn"`
	Friends              []string         `json:"friends"`
	CompilerArgs         []string         `json:"compilerArgs"`
	JVMTarget            string           `json:"jvmTarget,omitempty"`
	Fragments            []ModuleFragment `json:"fragments,omitempty"`
}

// AnalyzeModulesRequest is the protocol-only module check request. Consumers
// opt into it explicitly; the legacy check producer remains unchanged.
type AnalyzeModulesRequest struct {
	ID          int64             `json:"id"`
	Command     string            `json:"command"`
	Modules     []ModuleSpec      `json:"modules"`
	CheckFiles  []string          `json:"checkFiles"`
	Rules       []string          `json:"rules,omitempty"`
	RuleConfigs map[string]any    `json:"ruleConfigs,omitempty"`
	TestFiles   []string          `json:"testFiles,omitempty"`
	ScanPaths   map[string]string `json:"scanPaths,omitempty"`
	// SDKLevels is firDaemonRequest.SDKLevels for the module request.
	SDKLevels map[string]android.SDKLevels `json:"sdkLevels,omitempty"`
}

// ModuleStatus records the compilation mode, and first diagnostic if gated.
type ModuleStatus struct {
	ID         string `json:"id"`
	Mode       string `json:"mode"`
	FirstError string `json:"firstError,omitempty"`
}

// AnalyzeModulesResponse extends the existing file-keyed verdict envelope.
// DecidingModules maps each checked file to the module that decided its verdict.
type AnalyzeModulesResponse struct {
	CheckResponse
	Modules         []ModuleStatus    `json:"modules"`
	DecidingModules map[string]string `json:"decidingModules"`
}

// ModuleID is the stable module identifier of one Gradle source set.
func ModuleID(project, sourceSet string) string { return project + ":" + sourceSet }

// ModuleSpecsFromModel builds one ModuleSpec per Gradle source set. Each spec
// carries only its own source set's compiler arguments: the settings of one
// module are never applied to another, and a source set the model said nothing
// about gets no compiler arguments (the embedded compiler's defaults), exactly
// like the whole-repo path. Krit-owned arguments cannot reach CompilerArgs:
// they are rendered from KotlinSettings, which only holds allowlisted
// settings. Test and androidTest source sets see their project's main source
// set as a friend, and project dependencies map to the dependency's main
// source set when the model exported one.
func ModuleSpecsFromModel(m *gradlemodel.Model) []ModuleSpec {
	if m == nil {
		return nil
	}
	mainOf := map[string]string{}
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			if s.Kind == "main" {
				mainOf[p.Path] = ModuleID(p.Path, s.Name)
				break
			}
		}
	}
	var specs []ModuleSpec
	for _, p := range m.Projects {
		for _, s := range p.SourceSets {
			spec := ModuleSpec{
				ID:                   ModuleID(p.Path, s.Name),
				Platform:             s.Platform,
				Kind:                 s.Kind,
				SourceRoots:          nonNil(s.SourceDirs),
				GeneratedSourceRoots: nonNil(s.GeneratedSourceDirs),
				Classpath:            moduleClasspath(s),
				DependsOn:            []string{},
				Friends:              []string{},
				CompilerArgs:         nonNil(gradlemodel.ResolveKotlinSettings(s.Kotlin).CompilerArgs()),
				JVMTarget:            s.JvmTarget,
			}
			for _, dep := range s.ProjectDeps {
				if id, ok := mainOf[dep]; ok && id != spec.ID {
					spec.DependsOn = append(spec.DependsOn, id)
				}
			}
			if s.Kind != "main" {
				if id, ok := mainOf[p.Path]; ok {
					spec.DependsOn = append(spec.DependsOn, id)
					spec.Friends = append(spec.Friends, id)
				}
			}
			specs = append(specs, spec)
		}
	}
	return specs
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// moduleClasspath is the source set's boot, ordinary and generated classpath,
// deduplicated, in the order the whole-repo classpath uses.
func moduleClasspath(s gradlemodel.SourceSet) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, list := range [][]string{s.BootClasspath, s.ClasspathEntries, s.GeneratedClasspath} {
		for _, entry := range list {
			if entry != "" && !seen[entry] {
				seen[entry] = true
				out = append(out, entry)
			}
		}
	}
	return out
}
