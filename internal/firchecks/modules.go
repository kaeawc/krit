package firchecks

import "github.com/kaeawc/krit/internal/android"

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
