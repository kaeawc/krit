// Package jdk locates JDK command-line tools. JAVA_HOME takes precedence over
// PATH, so every JVM krit launches and every version probe it runs use the
// same installation. A build system can then pin the JDK through JAVA_HOME
// alone, without also controlling PATH.
package jdk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// HomeEnv names the JDK installation krit uses when it is set.
const HomeEnv = "JAVA_HOME"

// Tool returns the path of the named JDK tool, such as "java" or "javac".
// With JAVA_HOME set, the tool must exist under $JAVA_HOME/bin: a JAVA_HOME
// that lacks it is an error rather than a silent fall back to PATH, so a
// pinned JDK is never swapped for whatever PATH holds.
func Tool(name string) (string, error) {
	if home := os.Getenv(HomeEnv); home != "" {
		path := filepath.Join(home, "bin", executableName(name))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("%s does not contain bin/%s: %s", HomeEnv, executableName(name), home)
	}
	return exec.LookPath(name)
}

// Java returns the java launcher. See Tool.
func Java() (string, error) { return Tool("java") }

// Javac returns the javac compiler. See Tool.
func Javac() (string, error) { return Tool("javac") }

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
