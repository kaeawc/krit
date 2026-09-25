package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// kmpFixture is a Kotlin Multiplatform project with one expect function and
// class in commonMain and an actual for each of jvmMain, jsMain, nativeMain.
// The oracle compilation is a JVM compilation, so only commonMain and jvmMain
// belong in it. Compiling all four as one flat JVM module makes the call to
// platformLabel() ambiguous and loses the warnings at Common.kt:30 and :31.
//
// concurrentMain is a custom intermediate source set that compiles on the
// JVM: it declares an expect whose actual lives in jvmMain, so it must stay in
// the compilation and be compiled as a common source.
var kmpFixture = map[string]string{
	"src/commonMain/kotlin/repro/Common.kt": `package repro

expect fun platformLabel(): String

expect class PlatformBox {
    fun text(): String
}

// Expected UNNECESSARY_SAFE_CALL: the receiver is declared non-null.
fun commonSafeCall(): Int {
    val value: String = "ready"
    return value?.length ?: 0
}

// Expected USELESS_ELVIS: the left operand is declared non-null.
fun commonElvis(): String {
    val value: String = "ready"
    return value ?: "fallback"
}

// Resolution of trim() depends on kotlin-stdlib being on the oracle classpath.
// Expected UNNECESSARY_SAFE_CALL with stdlib present.
fun commonStdlibCase(): Int {
    val trimmed = " ready ".trim()
    return trimmed?.length ?: 0
}

// These depend on resolution of the expect declaration. A correctly scoped
// common+jvm compilation should report UNNECESSARY_SAFE_CALL and USELESS_ELVIS.
fun expectResultSafeCall(): Int = platformLabel()?.length ?: 0
fun expectResultElvis(): String = platformLabel() ?: "fallback"
`,
	"src/concurrentMain/kotlin/repro/Concurrent.kt": `package repro

expect fun lockLabel(): String

fun concurrentSafe(): Int = lockLabel()?.length ?: 0
`,
	"src/jvmMain/kotlin/repro/Platform.kt": `package repro

import java.io.File

actual fun platformLabel(): String = File(".").absolutePath

actual class PlatformBox {
    actual fun text(): String = File(".").name
}

actual fun lockLabel(): String = "jvm"

// JVM-only API: valid for jvmMain, unavailable to common/JS/Native targets.
fun jvmOnly(): String = File(".").canonicalPath
`,
	"src/jsMain/kotlin/repro/Platform.kt": `package repro

actual fun platformLabel(): String = js("'js'") as String

actual class PlatformBox {
    actual fun text(): String = js("'js box'") as String
}
`,
	"src/nativeMain/kotlin/repro/Platform.kt": `package repro

import platform.posix.getpid

actual fun platformLabel(): String = getpid().toString()

actual class PlatformBox {
    actual fun text(): String = getpid().toString()
}
`,
	"src/androidNativeArm64Main/kotlin/repro/Extra.kt": `package repro

actual fun lockLabel(): String = "android-native"
`,
}

// TestKMPSourceSetParity pins both oracle backends on a KMP project against
// kotlinc's JVM compilation of it (K2JVMCompiler with -Xmulti-platform
// -Xexpect-actual-classes -Xcommon-sources=<commonMain and concurrentMain
// files> over the commonMain, concurrentMain, and jvmMain files), which
// reports exactly the warnings in wantFIR below and no errors.
//
// krit-fir must match that ground truth. krit-types does NOT reach parity:
// the source-set filter removes the conflicting JS/Native actuals, but it
// still analyzes every root as one Analysis API module with no common /
// platform split, so every call through an expect declaration stays
// unresolved and loses its warning. That gap is asserted explicitly
// (wantKAA) rather than skipped, so a change in either direction is noticed;
// docs/oracle-kmp-source-sets.md documents it.
func TestKMPSourceSetParity(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar` to enable")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}

	project := t.TempDir()
	for rel, body := range kmpFixture {
		path := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, body)
	}

	sourceDirs := oracle.FindSourceDirs([]string{project})
	var sets []string
	for _, dir := range sourceDirs {
		sets = append(sets, filepath.Base(filepath.Dir(dir)))
	}
	sort.Strings(sets)
	if want := []string{"commonMain", "concurrentMain", "jvmMain"}; !reflect.DeepEqual(sets, want) {
		t.Fatalf("oracle source sets = %v, want %v", sets, want)
	}

	opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
	tmp := t.TempDir()
	kaa := runOneShotDirs(t, kaaJar, sourceDirs, filepath.Join(tmp, "kaa.json"), opts)
	fir := runOneShotDirs(t, firJar, sourceDirs, filepath.Join(tmp, "fir.json"), opts)

	wantFIR := []string{
		"Common.kt:12:UNNECESSARY_SAFE_CALL",
		"Common.kt:18:USELESS_ELVIS",
		"Common.kt:25:UNNECESSARY_SAFE_CALL",
		"Common.kt:30:UNNECESSARY_SAFE_CALL",
		"Common.kt:31:USELESS_ELVIS",
		"Concurrent.kt:5:UNNECESSARY_SAFE_CALL",
	}
	wantKAA := wantFIR[:3]
	wantFiles := []string{"commonMain/Common.kt", "concurrentMain/Concurrent.kt", "jvmMain/Platform.kt"}

	for _, tc := range []struct {
		name string
		data *oracle.Data
		want []string
	}{
		{"krit-fir", fir, wantFIR},
		{"krit-types", kaa, wantKAA},
	} {
		if got := oracleFileSets(tc.data); !reflect.DeepEqual(got, wantFiles) {
			t.Errorf("%s returned facts for %v, want only %v", tc.name, got, wantFiles)
		}
		if got := oracleDiagnostics(tc.data); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s diagnostics = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func runOneShotDirs(t *testing.T, jar string, sourceDirs []string, outputPath string, opts oracle.InvocationOptions) *oracle.Data {
	t.Helper()
	if _, err := oracle.InvokeWithFilesWithOptions(jar, sourceDirs, outputPath, "", false, opts); err != nil {
		t.Fatalf("oracle invoke (%s): %v", jar, err)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read %s: %v", outputPath, err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse %s: %v", outputPath, err)
	}
	return &data
}

// oracleFileSets names each analyzed file as <sourceSet>/<basename>.
func oracleFileSets(data *oracle.Data) []string {
	var out []string
	for path := range data.Files {
		set := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		out = append(out, set+"/"+filepath.Base(path))
	}
	sort.Strings(out)
	return out
}

func oracleDiagnostics(data *oracle.Data) []string {
	var out []string
	for path, file := range data.Files {
		for _, d := range file.Diagnostics {
			out = append(out, filepath.Base(path)+":"+strconv.Itoa(d.Line)+":"+d.FactoryName)
		}
	}
	sort.Strings(out)
	return out
}
