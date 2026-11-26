package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// runAxlRuleWithResolver runs one registered rule on the first of several Kotlin sources, with a type resolver
// indexed over all of them.
func runAxlRuleWithResolver(t *testing.T, ruleID string, sources map[string]string, target string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var files []*scanner.File
	var targetFile *scanner.File
	for name, src := range sources {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
		if name == target {
			targetFile = file
		}
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 1)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			cols := NewDispatcher([]*api.Rule{r}, resolver).Run(targetFile)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %s not registered", ruleID)
	return nil
}

// The resolver's class hierarchy proves a ViewModel declared in another file, which the per-file parity harness
// cannot (see testdata/androidx_lifecycle/compose/divergences.txt).
func TestViewModelConstructorInComposableAcrossFiles(t *testing.T) {
	models := `package com.example

import androidx.lifecycle.ViewModel

open class BaseViewModel : ViewModel()
class ChildViewModel : BaseViewModel()
class Plain
`
	screen := `package com.example

import androidx.compose.runtime.Composable

@Composable
fun Screen() {
    val vm = ChildViewModel()
    val plain = Plain()
}
`
	got := runAxlRuleWithResolver(t, "ViewModelConstructorInComposable",
		map[string]string{"Models.kt": models, "Screen.kt": screen}, "Screen.kt")
	if len(got) != 1 || got[0].Line != 7 {
		t.Fatalf("want one finding on line 7 (ChildViewModel), got %v", got)
	}
}

// super.onStart() reaches the library method, so it is not followed into the class's own onStart again.
func TestRepeatOnLifecycleSuperCallNotFollowed(t *testing.T) {
	src := `package com.example

import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch

class F : Fragment() {
    override fun onStart() {
        super.onStart()
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
        this.helper()
    }

    fun helper() {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) {} }
    }
}
`
	got := runAxlRuleWithResolver(t, "RepeatOnLifecycleWrongUsage", map[string]string{"F.kt": src}, "F.kt")
	if len(got) != 2 {
		t.Fatalf("want 2 findings (onStart and this.helper()), got %v", got)
	}
}
