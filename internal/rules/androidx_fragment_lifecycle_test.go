package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// fragmentLifecycleFindings runs one rule over a single source file with the source resolver, returning sorted
// "line:col message" entries.
func fragmentLifecycleFindings(t *testing.T, ruleID, name, src string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src", "main", "java", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	parse := scanner.ParseFile
	if strings.HasSuffix(path, ".java") {
		parse = scanner.ParseJavaFile
	}
	file, err := parse(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	rule := buildRuleIndex()[ruleID]
	if rule == nil {
		t.Fatalf("%s not registered", ruleID)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, resolver)
	cols := dispatcher.Run(file)
	var out []string
	for _, f := range cols.Findings() {
		out = append(out, fmt.Sprintf("%d:%d %s", f.Line, f.Col, f.Message))
	}
	sort.Strings(out)
	return out
}

const fragmentLifecycleSample = `package com.example

import android.os.Bundle
import android.view.Menu
import android.view.MenuInflater
import android.view.MenuItem
import android.view.View
import androidx.activity.OnBackPressedCallback
import androidx.activity.addCallback
import androidx.core.view.MenuProvider
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
import androidx.lifecycle.Observer
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.launch

class Model { val items: LiveData<Int> = MutableLiveData(1) }

val callback = object : OnBackPressedCallback(true) { override fun handleOnBackPressed() {} }
val provider = object : MenuProvider {
    override fun onCreateMenu(menu: Menu, inflater: MenuInflater) {}
    override fun onMenuItemSelected(item: MenuItem) = false
}

class MyFragment : Fragment() {
    val live = MutableLiveData<Int>()
    val model = Model()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        live.observe(this) { }
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        live.observe(this) { }
        live.observe(viewLifecycleOwner) { }
        live.observe(this, Observer { })
        model.items.observe(this@MyFragment) { }
        requireActivity().onBackPressedDispatcher.addCallback(this, callback)
        requireActivity().onBackPressedDispatcher.addCallback(this) { }
        requireActivity().onBackPressedDispatcher.addCallback(callback)
        requireActivity().addMenuProvider(provider, this)
        requireActivity().addMenuProvider(provider, viewLifecycleOwner, Lifecycle.State.RESUMED)
        requireActivity().addMenuProvider(provider, this, Lifecycle.State.RESUMED)
        helper()
        Helper().watch(live, this)
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
        viewLifecycleOwner.lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
    }

    fun helper() {
        live.observe(this) { }
    }
}

class Helper {
    fun watch(data: LiveData<Int>, fragment: MyFragment) {
        data.observe(fragment) { }
    }
}

class MyDialog : DialogFragment() {
    val live = MutableLiveData<Int>()
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        live.observe(this) { }
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
    }
}

class Other : Fragment() {
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
    }
    override fun onCreateView(inflater: android.view.LayoutInflater, c: android.view.ViewGroup?, s: Bundle?): View? {
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
        val unused = viewLifecycleOwner
        return null
    }
    override fun onStart() {
        super.onStart()
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
    }
}
`

const fragmentLifecycleJavaSample = `package com.example;

import android.os.Bundle;
import android.view.View;
import androidx.fragment.app.Fragment;
import androidx.lifecycle.MutableLiveData;

public class J extends Fragment {
    MutableLiveData<Integer> live = new MutableLiveData<>();

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        live.observe(this, x -> { });
        live.observe(getViewLifecycleOwner(), x -> { });
    }
}
`

// Expected positions and messages come from running the lint.jar of androidx.fragment:fragment 1.9.1 through the
// Android Lint CLI on the same files, with the AndroidX classes on the classpath. Lint follows the same-file helper()
// call (56:22) and the Helper().watch(live, this) call, reporting the observe inside Helper at the call (62:9); it does
// not report onCreate, viewLifecycleOwner, the addCallback(this) { } extension, the one-argument addCallback, or
// DialogFragment owners, and repeatOnLifecycle only where the method body never mentions viewLifecycleOwner.
func TestFragmentLifecycleOwnerLintParity(t *testing.T) {
	const use = "Use viewLifecycleOwner as the LifecycleOwner."
	want := map[string][]string{
		"FragmentLiveDataObserve": {"39:22 " + use, "41:22 " + use, "42:29 " + use, "56:22 " + use,
			"62:9 Unsafe call to observe with Fragment instance as LifecycleOwner from MyFragment.onViewCreated."},
		"FragmentBackPressedCallback":     {"43:63 " + use},
		"FragmentAddMenuProvider":         {"46:53 " + use, "48:53 " + use},
		"UnsafeRepeatOnLifecycleDetector": {"76:33 The repeatOnLifecycle API should be used with viewLifecycleOwner"},
	}
	for rule, expected := range want {
		t.Run(rule, func(t *testing.T) {
			sort.Strings(expected)
			if got := fragmentLifecycleFindings(t, rule, "com/example/F.kt", fragmentLifecycleSample); !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
	if got := fragmentLifecycleFindings(t, "FragmentLiveDataObserve", "com/example/J.java", fragmentLifecycleJavaSample); !reflect.DeepEqual(got,
		[]string{"13:22 Use getViewLifecycleOwner() as the LifecycleOwner."}) {
		t.Fatalf("Java: got %q", got)
	}
}

// Without the type oracle, a LiveData declared in another file (the usual ViewModel property) cannot be proven, so the
// Go path does not report it; Lint and the oracle do. Pinned as a known false negative.
func TestFragmentLiveDataObserveCrossFileReceiverIsAKnownFalseNegative(t *testing.T) {
	src := `package com.example

import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment

class FeedFragment : Fragment() {
    private val viewModel = FeedViewModel()

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        viewModel.items.observe(this) { }
    }
}
`
	if got := fragmentLifecycleFindings(t, "FragmentLiveDataObserve", "com/example/FeedFragment.kt", src); len(got) != 0 {
		t.Fatalf("got %q, want none without the oracle", got)
	}
}

// this inside apply/with refers to the scope subject, so it is not the Fragment; a plain lambda keeps the Fragment as
// this. Inside an object expression the call's containing class is the object, so Lint (and krit) report the Fragment
// passed as this@FeedFragment at the call instead of at the argument. Positions come from the Lint CLI.
func TestFragmentLifecycleOwnerThisBinding(t *testing.T) {
	src := `package com.example

import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.MutableLiveData

class Owner : LifecycleOwner {
    override val lifecycle get() = TODO()
}

class FeedFragment : Fragment() {
    private val live = MutableLiveData(0)

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        Owner().apply { live.observe(this) { } }
        view.post(object : Runnable {
            override fun run() {
                live.observe(this@FeedFragment) { }
            }
        })
        view.post { live.observe(this) { } }
    }
}
`
	const use = "Use viewLifecycleOwner as the LifecycleOwner."
	got := fragmentLifecycleFindings(t, "FragmentLiveDataObserve", "com/example/FeedFragment.kt", src)
	if want := []string{"20:17 Unsafe call to observe with Fragment instance as LifecycleOwner from FeedFragment.onViewCreated.",
		"23:34 " + use}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Lint passes the launch-scope test when viewLifecycleOwner appears anywhere in the method body, comments included,
// and fails it when the call enclosing repeatOnLifecycle has no receiver. krit keeps both verdicts.
func TestUnsafeRepeatOnLifecycleMatchesLintsScopeText(t *testing.T) {
	src := `package com.example

import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch

class A : Fragment() {
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        // TODO: move to viewLifecycleOwner
        lifecycleScope.launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
    }
}

class B : Fragment() {
    override fun onViewStateRestored(savedInstanceState: Bundle?) {
        lifecycleScope.launch {
            coroutineScope {
                launch { repeatOnLifecycle(Lifecycle.State.STARTED) { } }
            }
        }
        val owner = viewLifecycleOwner
    }
}
`
	got := fragmentLifecycleFindings(t, "UnsafeRepeatOnLifecycleDetector", "com/example/A.kt", src)
	if want := []string{"23:26 The repeatOnLifecycle API should be used with viewLifecycleOwner"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
