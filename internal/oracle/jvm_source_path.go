package oracle

import "path/filepath"

// InJVMCompilableSourceSet reports whether a source file belongs in a JVM
// compilation. A file under a Gradle source-set root (`.../src/<set>/kotlin`
// or `java`) is JVM-compilable exactly when FindSourceDirs would keep that
// root (see isJVMCompilableSourceSet); a file outside that layout is kept,
// since its target cannot be read from the path.
func InJVMCompilableSourceSet(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	for dir := filepath.Dir(abs); ; {
		base := filepath.Base(dir)
		if (base == "kotlin" || base == "java") && filepath.Base(filepath.Dir(filepath.Dir(dir))) == "src" {
			return isJVMCompilableSourceSet(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return true
		}
		dir = parent
	}
}
