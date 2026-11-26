package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// manifestSourceSecurityRules are the manifest-source Security ports that index every project class.
var manifestSourceSecurityRules = []string{"ProviderReadPermissionOnly", "UnsafeImplicitIntentLaunch", "UnsafeIntentLaunch"}

// manifestSourceScaleProject lays out modules app modules, each with a manifest that exports its activity, service
// and receiver, and sources whose superclass chains run through other files of the module. Half the manifests have
// no package, so components resolve through the Gradle-namespace fallback.
func manifestSourceScaleProject(tb testing.TB, modules int) ([]*rules.Manifest, []*scanner.File) {
	tb.Helper()
	dir := tb.TempDir()
	var manifests []*rules.Manifest
	var files []*scanner.File
	write := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			tb.Fatal(err)
		}
		files = append(files, f)
	}
	for m := 0; m < modules; m++ {
		pkg := fmt.Sprintf("com.scale.m%d", m)
		// Component names are unique per module, as the no-package fallback needs.
		n := fmt.Sprint(m)
		src := filepath.Join(dir, fmt.Sprintf("m%d", m), "src", "main", "java", filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")))
		write(filepath.Join(src, "Base.kt"), "package "+pkg+"\n\nimport androidx.appcompat.app.AppCompatActivity\n\n"+
			"open class BaseActivity"+n+" : AppCompatActivity()\nopen class Model(val id: String) {\n    class Nested(id: String) : Model(id)\n}\n")
		write(filepath.Join(src, "Screen.kt"), "package "+pkg+"\n\nimport android.content.Intent\nimport android.os.Bundle\n\n"+
			"class ScreenActivity"+n+" : BaseActivity"+n+"() {\n"+
			"    override fun onCreate(savedInstanceState: Bundle?) {\n"+
			"        super.onCreate(savedInstanceState)\n"+
			"        val forward = intent.getParcelableExtra<Intent>(\"next\")\n"+
			"        if (forward != null) startActivity(forward)\n"+
			"    }\n"+
			"    override fun onNewIntent(intent: Intent) {\n        super.onNewIntent(intent)\n        handle(intent)\n    }\n"+
			"    private fun handle(i: Intent) { setResult(0, i) }\n}\n")
		write(filepath.Join(src, "Worker.kt"), "package "+pkg+"\n\nimport android.app.Service\nimport android.content.Intent\n"+
			"import android.os.IBinder\n\nclass WorkerService"+n+" : Service() {\n"+
			"    override fun onBind(intent: Intent?): IBinder? = null\n"+
			"    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {\n"+
			"        intent?.let { startService(it) }\n        return START_NOT_STICKY\n    }\n}\n")
		write(filepath.Join(src, "Event.kt"), "package "+pkg+"\n\nimport android.content.BroadcastReceiver\n"+
			"import android.content.Context\nimport android.content.Intent\n\nclass EventReceiver"+n+" : BroadcastReceiver() {\n"+
			"    override fun onReceive(context: Context, intent: Intent) {\n        context.startActivity(intent)\n    }\n}\n")
		write(filepath.Join(src, "Data.kt"), "package "+pkg+"\n\nimport android.content.ContentProvider\n\n"+
			"abstract class DataProvider"+n+" : ContentProvider()\ninterface Repo { fun load(id: String): String }\n"+
			"class RepoImpl : Repo {\n    override fun load(id: String): String = id\n}\n")
		pkgAttr := ` package="` + pkg + `"`
		if m%2 == 1 {
			pkgAttr = ""
		}
		manifests = append(manifests, parseManifestAt(tb, filepath.Join(dir, fmt.Sprintf("m%d", m), "src", "main", "AndroidManifest.xml"),
			`<manifest xmlns:android="http://schemas.android.com/apk/res/android"`+pkgAttr+`>
    <application>
        <activity android:name=".ScreenActivity`+n+`" android:exported="true" />
        <service android:name=".WorkerService`+n+`" android:exported="true" />
        <receiver android:name=".EventReceiver`+n+`" android:exported="true" />
        <provider android:name=".DataProvider`+n+`" android:authorities="x" android:readPermission="p" />
    </application>
</manifest>
`))
	}
	return manifests, files
}

func manifestSourceSecurityDispatcher(tb testing.TB) *rules.Dispatcher {
	tb.Helper()
	var active []*api.Rule
	for _, id := range manifestSourceSecurityRules {
		for _, r := range api.Registry {
			if r.ID == id {
				active = append(active, r)
			}
		}
	}
	if len(active) != len(manifestSourceSecurityRules) {
		tb.Fatalf("found %d of the manifest-source Security rules", len(active))
	}
	return rules.NewDispatcher(active, nil)
}

// manifestSourceBytes returns the bytes allocated by one run of the rules, and the number of findings.
func manifestSourceBytes(t *testing.T, modules int) (uint64, int) {
	t.Helper()
	manifests, files := manifestSourceScaleProject(t, modules)
	d := manifestSourceSecurityDispatcher(t)
	// Warm the files' lazily built caches so only the rules' own work is measured.
	d.RunManifestSource(manifests, files)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	cols, ok := d.RunManifestSource(manifests, files)
	runtime.ReadMemStats(&after)
	if !ok {
		t.Fatal("RunManifestSource recovered a panic")
	}
	return after.TotalAlloc - before.TotalAlloc, cols.Len()
}

// TestManifestSourceSecurityRulesScaleLinearly guards #1658: the rules once rebuilt a map of every project class
// on each superclass query, so their work grew with classes × queries. Allocation is a deterministic proxy for that
// work: with 4x the modules a linear implementation allocates about 4x the bytes, and the quadratic one about 16x.
func TestManifestSourceSecurityRulesScaleLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a few hundred files")
	}
	const small, large = 24, 96
	smallBytes, smallFindings := manifestSourceBytes(t, small)
	largeBytes, largeFindings := manifestSourceBytes(t, large)
	if smallFindings == 0 || largeFindings != smallFindings*large/small {
		t.Fatalf("findings: %d modules -> %d, %d modules -> %d; want a fixed number per module", small, smallFindings,
			large, largeFindings)
	}
	ratio := float64(largeBytes) / float64(smallBytes)
	t.Logf("allocated %d bytes for %d modules, %d bytes for %d modules (ratio %.1f)", smallBytes, small, largeBytes,
		large, ratio)
	if ratio > 7 {
		t.Fatalf("allocation grew %.1fx for %dx the modules; the rules are doing per-query work over the whole project",
			ratio, large/small)
	}
}

// BenchmarkManifestSourceSecurityRules times the three rules over a 400-module project (2,800 classes).
func BenchmarkManifestSourceSecurityRules(b *testing.B) {
	manifests, files := manifestSourceScaleProject(b, 400)
	d := manifestSourceSecurityDispatcher(b)
	b.ReportAllocs()
	for b.Loop() {
		d.RunManifestSource(manifests, files)
	}
}
