package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAndroidxLifecycleWarmRunAfterOtherFileEdit runs a cold scan, changes a supertype in *another* file, and checks
// that the warm (cached) scan reports exactly what an uncached scan does for the lifecycle rules whose verdicts
// depend on class ancestry (#1759 review; #1778). The per-file findings cache is keyed by the file's own content,
// so these rules must not read hierarchy facts from other files.
func TestAndroidxLifecycleWarmRunAfterOtherFileEdit(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/main/kotlin/p/Models.kt", `package p

import androidx.lifecycle.ViewModel

open class BaseViewModel : ViewModel()
class ChildViewModel : BaseViewModel()
`)
	write("src/main/kotlin/p/Screen.kt", `package p

import androidx.compose.runtime.Composable
import androidx.lifecycle.ViewModel

class LocalViewModel : ViewModel()

@Composable
fun Screen() {
    val vm = ChildViewModel()
    val local = LocalViewModel()
}
`)
	write("src/main/kotlin/p/Base.kt", `package p

import androidx.fragment.app.Fragment

open class BaseFragment : Fragment()
`)
	write("src/main/kotlin/p/Frag.kt", `package p

import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch

class MyFragment : BaseFragment() {
    override fun onStart() {
        super.onStart()
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }
}
`)
	rules := map[string]bool{"ViewModelConstructorInComposable": true, "RepeatOnLifecycleWrongUsage": true}
	scan := func(extra ...string) []string {
		t.Helper()
		args := append([]string{"--no-daemon", "--no-type-oracle", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit exited %d: %s", code, stderr)
		}
		var report struct {
			Findings []scopeFinding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout)
		}
		var keys []string
		for _, f := range report.Findings {
			if rules[f.Rule] {
				keys = append(keys, fmt.Sprintf("%s %s:%d", f.Rule, filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return keys
	}
	cold := scan()
	if strings.Join(cold, "\n") != "ViewModelConstructorInComposable Screen.kt:11" {
		t.Fatalf("cold scan: want only the same-file LocalViewModel, got %v", cold)
	}
	// The other files' classes stop being a ViewModel and a Fragment; Screen.kt and Frag.kt are unchanged.
	write("src/main/kotlin/p/Models.kt", "package p\n\nopen class BaseViewModel\nclass ChildViewModel : BaseViewModel()\n")
	write("src/main/kotlin/p/Base.kt", "package p\n\nopen class BaseFragment\n")
	warm := scan()
	uncached := scan("--no-cache")
	if strings.Join(warm, "\n") != strings.Join(uncached, "\n") {
		t.Fatalf("warm scan after another file changed differs from --no-cache:\nwarm:     %v\nno-cache: %v", warm, uncached)
	}
}
