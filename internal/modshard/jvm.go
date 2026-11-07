package modshard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kaeawc/krit/internal/jvmaot"
	"github.com/kaeawc/krit/internal/oracle"
)

// workerJVM is the JVM the module workers run on and the Leyden AOT arguments they share.
type workerJVM struct {
	java string
	// aot holds the arguments that apply an AOT cache, or record a profile when none exists yet.
	aot []string
	// recording reports whether aot records a profile (then only the recording worker gets it).
	recording bool
}

// argsFor returns the AOT arguments for a worker shard: a ready cache applies to every worker, a
// recording to the first only.
func (j workerJVM) argsFor(shard int) []string {
	if j.recording && shard != 0 {
		return nil
	}
	return j.aot
}

// minAOTJDK is the first JDK whose Leyden AOT cache covers the compiler's startup and warm-up.
const minAOTJDK = 25

// chooseWorkerJVM picks the JVM for the workers, preferring a JDK that supports Leyden AOT, and
// prepares the AOT arguments for it. Without such a JDK, or with KRIT_WORKER_AOT=0, the workers run on
// in.JavaPath without AOT.
func chooseWorkerJVM(in Input) workerJVM {
	if os.Getenv("KRIT_WORKER_AOT") == "0" {
		return workerJVM{java: in.JavaPath}
	}
	java := preferAOTJava(in.JavaPath)
	major := oracle.JavaMajorVersion(java)
	jvm := workerJVM{java: java}
	if major < minAOTJDK {
		return jvm
	}
	args, used := jvmaot.AppendArgs(nil, java, in.JarPath, "module-facts", major, in.Logf != nil, in.Logf)
	if used {
		jvm.aot, jvm.recording = args, jvmaot.IsRecording(args)
	}
	return jvm
}

// preferAOTJava returns a JDK of at least minAOTJDK when the machine has one, else fallback. The
// KRIT_WORKER_JAVA environment variable names one explicitly.
func preferAOTJava(fallback string) string {
	for _, candidate := range javaCandidates() {
		if candidate != "" && oracle.JavaMajorVersion(candidate) >= minAOTJDK {
			return candidate
		}
	}
	return fallback
}

func javaCandidates() []string {
	candidates := []string{os.Getenv("KRIT_WORKER_JAVA")}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, "bin", "java"))
	}
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.CommandContext(context.Background(), "/usr/libexec/java_home", "-v", "25+").Output(); err == nil {
			candidates = append(candidates, filepath.Join(strings.TrimSpace(string(out)), "bin", "java"))
		}
	case "linux":
		if matches, _ := filepath.Glob("/usr/lib/jvm/*/bin/java"); len(matches) > 0 {
			candidates = append(candidates, matches...)
		}
	}
	return candidates
}
