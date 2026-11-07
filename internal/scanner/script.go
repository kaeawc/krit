package scanner

import "strings"

// ScriptKind classifies a Kotlin script file by how it is run.
type ScriptKind uint8

const (
	// ScriptNone is a regular Kotlin source file (.kt) or a non-Kotlin file.
	ScriptNone ScriptKind = iota
	// ScriptGradle is a Gradle Kotlin DSL script: build.gradle.kts,
	// settings.gradle.kts, init.gradle.kts and precompiled script plugins
	// (<name>.gradle.kts).
	ScriptGradle
	// ScriptStandalone is any other .kts file: <name>.main.kts and scripts
	// with a custom script definition. They run on their own, outside Gradle.
	ScriptStandalone
)

// String returns a short human-readable name for the kind.
func (k ScriptKind) String() string {
	switch k {
	case ScriptGradle:
		return "gradle"
	case ScriptStandalone:
		return "standalone"
	default:
		return "none"
	}
}

// ScriptKindOf classifies path by its file name, with the same case-sensitive
// .kts match the file walker uses. Paths that do not end in .kts are ScriptNone.
func ScriptKindOf(path string) ScriptKind {
	switch {
	case strings.HasSuffix(path, ".gradle.kts"):
		return ScriptGradle
	case strings.HasSuffix(path, ".kts"):
		return ScriptStandalone
	default:
		return ScriptNone
	}
}

// IsKotlinScriptPath reports whether path names a Kotlin script (.kts).
func IsKotlinScriptPath(path string) bool {
	return ScriptKindOf(path) != ScriptNone
}

// ScriptKind returns the script kind of a Kotlin-language file. Gradle-language
// files and non-script files report ScriptNone, so callers can gate script
// policy on the result alone.
func (f *File) ScriptKind() ScriptKind {
	if f == nil || f.Language != LangKotlin {
		return ScriptNone
	}
	return ScriptKindOf(f.Path)
}

// IsScript reports whether f is a Kotlin script parsed as Kotlin source.
func (f *File) IsScript() bool {
	return f.ScriptKind() != ScriptNone
}
