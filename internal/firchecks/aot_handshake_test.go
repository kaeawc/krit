package firchecks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/jvmaot"
)

func TestFirJVMArgsUseLeydenCacheWhenSupported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cachePath, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("trained cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("trained cache"))
	data, err := json.Marshal(map[string]any{
		"schema": 1, "jdkVersion": "major:25", "jarToken": buildid.JarToken(jar),
		"workload": "fir", "exitStatus": 0, "size": len("trained cache"),
		"sha256": hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath+".meta.json", data, 0o644); err != nil {
		t.Fatal(err)
	}

	args := buildFirJVMArgs(jar, "missing-java", 25)
	if !containsJVMArg(args, "-XX:AOTCache="+cachePath) {
		t.Fatalf("supported JDK args missing AOT cache %q: %v", cachePath, args)
	}
	if !containsJVMArg(args, "-Xmx1g") {
		t.Fatalf("checker args missing oracle-aligned heap cap: %v", args)
	}
}

func TestFirJVMArgsSkipLeydenOnUnsupportedJDK(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, cachePath, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("trained cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := buildFirJVMArgs(jar, "java", 24)
	for _, arg := range args {
		if strings.Contains(arg, "AOT") || strings.Contains(arg, "aot") {
			t.Fatalf("unsupported JDK args unexpectedly include AOT option %q: %v", arg, args)
		}
	}
}

func TestFirAOTCachePathUsesJarTokenAndWorkload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	write := func(content string) string {
		t.Helper()
		if err := os.WriteFile(jar, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return buildid.JarToken(jar)
	}
	token1 := write("first contents")
	_, firPath1, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	token2 := write("second contents")
	_, firPath2, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	if token1 == token2 || firPath1 == firPath2 {
		t.Fatalf("changing jar content token did not change checker path: token %q -> %q, path %q -> %q", token1, token2, firPath1, firPath2)
	}
	if !strings.Contains(filepath.Base(firPath1), token1[:12]) || !strings.Contains(filepath.Base(firPath2), token2[:12]) {
		t.Fatalf("checker paths do not encode JarToken prefixes: %q, %q", firPath1, firPath2)
	}

	// The oracle keeps its existing hash-based cache filename; the FIR suffix
	// disambiguates its workload and also its JarToken identity.
	oracleToken, err := hashutil.HashFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	oraclePath := filepath.Join(filepath.Dir(firPath2), "krit-types-"+oracleToken[:12]+".aot")
	if firPath2 == oraclePath {
		t.Fatalf("FIR and oracle AOT paths collide: %q", firPath2)
	}
}

func TestReadFirReadySkipsJVMStdoutNoise(t *testing.T) {
	input := "Picked up JAVA_TOOL_OPTIONS: -XX:StartFlightRecording=filename=x.jfr\n" +
		"[1.234s][info][jfr,startup] Started recording 1.\n" +
		"[1.235s][info][jfr,startup] Use jcmd to dump recording data.\n" +
		"{\"ready\":true,\"port\":51823}\n"
	ready, err := readFirReady(strings.NewReader(input), false)
	if err != nil {
		t.Fatalf("readFirReady rejected JVM noise: %v", err)
	}
	if !ready.Ready || ready.Port != 51823 {
		t.Fatalf("readFirReady = %+v, want ready port 51823", ready)
	}
}

func TestReadFirReadyBoundsStdoutNoise(t *testing.T) {
	input := strings.Repeat("Picked up JAVA_TOOL_OPTIONS\n", firReadyMaxSkippedLines+1)
	_, err := readFirReady(strings.NewReader(input), false)
	if err == nil || !strings.Contains(err.Error(), "stdout noise limit") {
		t.Fatalf("readFirReady error = %v, want clear stdout noise limit error", err)
	}
}

func containsJVMArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
