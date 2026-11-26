package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// Expected positions come from running Android Lint 32.4.1 on the same inputs.

func TestTileProviderPermissionsLintParity(t *testing.T) {
	xml := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="test.pkg">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34" />
    <application>
        <service android:name=".NoPermission" android:exported="true">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
        </service>
        <service
            android:name=".WrongPermission"
            android:exported="true"
            android:permission="android.permission.BIND_JOB_SERVICE">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
        </service>
        <service android:name=".Good" android:exported="true"
            android:permission="com.google.android.wearable.permission.BIND_TILE_PROVIDER">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
        </service>
        <service android:name=".Other" android:exported="true">
            <intent-filter>
                <action android:name="com.example.OTHER" />
            </intent-filter>
        </service>
        <service android:name=".Twice" android:exported="true">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
        </service>
        <activity android:name=".Act" android:exported="true">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	// Lint reports a service once per BIND_TILE_PROVIDER action, so .Twice is reported three times.
	if got, want := manifestRulePositions(t, "TileProviderPermissions", xml), "4:10,9:10,28:10,28:10,28:10"; got != want {
		t.Fatalf("TileProviderPermissions positions = %s, want %s", got, want)
	}
}

// nativeCodeFindings writes files under a src/main source set and runs UnsafeNativeCodeLocation over its res
// directory. It returns "path:line:col message-prefix" entries relative to src/main, sorted.
func nativeCodeFindings(t *testing.T, files map[string]string) []string {
	t.Helper()
	main := filepath.Join(t.TempDir(), "src", "main")
	resDir := filepath.Join(main, "res")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(main, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range runResourceRule(findResourceRule(t, "UnsafeNativeCodeLocation"), idx) {
		rel, _ := filepath.Rel(main, f.File)
		kind := "executable"
		if strings.HasPrefix(f.Message, "Shared libraries should not be placed") {
			kind = "shared"
		}
		out = append(out, fmt.Sprintf("%s:%d:%d %s", filepath.ToSlash(rel), f.Line, f.Col, kind))
	}
	sort.Strings(out)
	return out
}

func TestUnsafeNativeCodeLocationLintParity(t *testing.T) {
	const elf = "\x7fELF\x02\x01\x01\x00rest"
	got := nativeCodeFindings(t, map[string]string{
		"res/raw/libfoo.so":      elf,
		"res/raw/helper":         elf,
		"res/raw/fake.so":        "notelf",
		"res/raw/short":          "\x7fEL",
		"res/raw-v21/libbar.so":  elf,
		"res/drawable/x.so":      elf,
		"assets/tool":            elf,
		"assets/libz.so":         elf,
		"assets/sub/deep.so":     elf,
		"jniLibs/arm64/libok.so": elf,
	})
	want := []string{
		"assets/libz.so:1:1 shared",
		"assets/tool:1:1 executable",
		"res/raw-v21/libbar.so:1:1 shared",
		"res/raw/helper:1:1 executable",
		"res/raw/libfoo.so:1:1 shared",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("UnsafeNativeCodeLocation findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestUnsafeNativeCodeLocationMessagesMatchLint(t *testing.T) {
	main := filepath.Join(t.TempDir(), "src", "main")
	resDir := filepath.Join(main, "res")
	for _, rel := range []string{"res/raw/libfoo.so", "assets/tool"} {
		path := filepath.Join(main, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("\x7fELF"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	msgs := map[string]string{}
	for _, f := range runResourceRule(findResourceRule(t, "UnsafeNativeCodeLocation"), idx) {
		msgs[filepath.Base(f.File)] = f.Message
	}
	if want := "Shared libraries should not be placed in the res or assets directories. Please use the features of " +
		"your development environment to place shared libraries in the lib directory of the compiled APK."; msgs["libfoo.so"] != want {
		t.Errorf("shared library message = %q", msgs["libfoo.so"])
	}
	if want := "Embedding non-shared library native executables into applications should be avoided when possible, " +
		"as there is an increased risk that the executables could be tampered with after installation. Instead, native " +
		"code should be placed in a shared library, and the features of the development environment should be used to " +
		"place the shared library in the lib directory of the compiled APK."; msgs["tool"] != want {
		t.Errorf("executable message = %q", msgs["tool"])
	}
}

// #1619: Lint also reads the assets (and res/raw) folders of the module's other source sets, such as build types and
// flavors; test source sets are not packaged.
func TestUnsafeNativeCodeLocationReadsEverySourceSet(t *testing.T) {
	const elf = "\x7fELF\x02\x01\x01\x00rest"
	src := filepath.Join(t.TempDir(), "app", "src")
	for rel, body := range map[string]string{
		"main/res/values/strings.xml": "<resources/>",
		"main/assets/libmain.so":      elf,
		"debug/assets/tool":           elf,
		"free/assets/libfree.so":      elf,
		"debug/res/raw/libraw.so":     elf,
		"test/assets/libtest.so":      elf,
		"androidTest/assets/tool":     elf,
	} {
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(filepath.Join(src, "main", "res"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range runResourceRule(findResourceRule(t, "UnsafeNativeCodeLocation"), idx) {
		rel, _ := filepath.Rel(src, f.File)
		got = append(got, filepath.ToSlash(rel))
	}
	sort.Strings(got)
	want := []string{"debug/assets/tool", "debug/res/raw/libraw.so", "free/assets/libfree.so", "main/assets/libmain.so"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("findings %v, want %v", got, want)
	}
}

// The rule follows an asset symlink to an ELF binary (the res-dir cache key reads it the same way, see
// android.IsELFFile), and a symlink to a directory is not a file.
func TestUnsafeNativeCodeLocationFollowsSymlinkedAssets(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "app", "src")
	for rel, body := range map[string]string{
		"app/src/main/res/values/strings.xml": "<resources/>",
		"elsewhere/libreal.so":                "\x7fELF\x02\x01\x01\x00rest",
		"elsewhere/dir/libinner.so":           "\x7fELF\x02\x01\x01\x00rest",
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(src, "debug", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{"liblink.so": "elsewhere/libreal.so", "libdir.so": "elsewhere/dir"} {
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(src, "debug", "assets", link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	idx, err := android.ScanResourceDir(filepath.Join(src, "main", "res"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range runResourceRule(findResourceRule(t, "UnsafeNativeCodeLocation"), idx) {
		rel, _ := filepath.Rel(src, f.File)
		got = append(got, filepath.ToSlash(rel))
	}
	if strings.Join(got, ",") != "debug/assets/liblink.so" {
		t.Fatalf("findings %v, want [debug/assets/liblink.so]", got)
	}
}
