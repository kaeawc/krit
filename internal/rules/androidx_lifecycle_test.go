package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
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

// The per-file findings cache is keyed by the file's own content (#1778), so the ViewModel ancestry is proven from
// the file alone: a ViewModel declared in another file is not reported even when the resolver indexes it, and a
// same-file one is.
func TestViewModelConstructorInComposableIsFileLocal(t *testing.T) {
	models := `package com.example

import androidx.lifecycle.ViewModel

open class BaseViewModel : ViewModel()
class ChildViewModel : BaseViewModel()
`
	screen := `package com.example

import androidx.compose.runtime.Composable
import androidx.lifecycle.ViewModel

class LocalViewModel : ViewModel()

@Composable
fun Screen() {
    val vm = ChildViewModel()
    val local = LocalViewModel()
}
`
	got := runAxlRuleWithResolver(t, "ViewModelConstructorInComposable",
		map[string]string{"Models.kt": models, "Screen.kt": screen}, "Screen.kt")
	if len(got) != 1 || got[0].Line != 11 {
		t.Fatalf("want only the same-file LocalViewModel (line 11), got %v", got)
	}
}

// #1759 review (P2): a `.lifecycle` property of a same-file class that is not a Lifecycle is not reported.
func TestLifecycleCurrentStateNeedsLifecycleReceiver(t *testing.T) {
	src := `package com.example

import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.LifecycleOwner

class FakeLifecycle { val currentState = 0 }
class Screen { val lifecycle = FakeLifecycle() }

@Composable
fun Status(screen: Screen, owner: LifecycleOwner, other: Unknown) {
    Text("${screen.lifecycle.currentState}")
    Text("${owner.lifecycle.currentState}")
    Text("${other.lifecycle.currentState}")
}
`
	got := runAxlRule(t, "LifecycleCurrentStateInComposition", "S.kt", src, librarymodel.ProjectProfile{})
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	// Line 13 is a LifecycleOwner; line 14's owner type is not in the file, so the property name decides.
	if len(lines) != 2 || lines[0] != 13 || lines[1] != 14 {
		t.Fatalf("want findings on lines 13 and 14, got %v", lines)
	}
}

// #1759 review (P3): as in the detector, an assignment to a local, or of a local, does not reset the verdict.
func TestTestLifecycleOwnerInCoroutineLocalAssignments(t *testing.T) {
	src := `package com.example

import androidx.lifecycle.Lifecycle
import androidx.lifecycle.testing.TestLifecycleOwner
import kotlinx.coroutines.test.runTest

fun t() = runTest {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.RESUMED
    var x = 0
    val y = 1
    x = y
}
`
	if got := runAxlRule(t, "TestLifecycleOwnerInCoroutine", "T.kt", src, librarymodel.ProjectProfile{}); len(got) != 1 {
		t.Fatalf("want 1 finding, got %v", got)
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
