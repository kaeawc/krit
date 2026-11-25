package rules_test

import (
	"reflect"
	"sort"
	"testing"
)

const fragmentMiscSample = `package com.example

import android.app.Dialog
import android.content.Context
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment

class F : Fragment() {
    fun go(other: Fragment) {
        val c = context!!
        activity!!.finish()
        val a = requireNotNull(arguments)
        val b = checkNotNull(context)
        val d = requireNotNull(activity) { "no activity" }
        val v = view!!
        val p = parentFragment!!
        val h = host!!
        val o = other.context!!
        val t = this.context!!
        val g = getContext()!!
        val n = (context)!!
        val ctx: Context? = null
        val local = ctx!!
    }
    fun shadow() {
        val context: Context? = null
        context!!
    }
}

class Plain {
    val context: Context? = null
    fun f() = context!!
}

class D : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog {
        val view = LayoutInflater.from(requireContext()).inflate(0, null)
        val dialog = AlertDialog.Builder(requireContext()).create()
        dialog.setOnCancelListener { }
        dialog.setOnDismissListener { }
        return dialog
    }
    fun other() {
        AlertDialog.Builder(requireContext()).create().setOnDismissListener { }
    }
}

class Fx : Fragment() {
    fun inflateIt(): View = LayoutInflater.from(requireContext()).inflate(0, null)
    fun tx(f: Fragment, g: Fragment) {
        parentFragmentManager.beginTransaction().detach(f).attach(f).commit()
        val t = parentFragmentManager.beginTransaction()
        t.detach(g)
        t.attach(g)
        t.commit()
        childFragmentManager.beginTransaction().detach(f).attach(g).commit()
        parentFragmentManager.beginTransaction().detach(f).commit()
        parentFragmentManager.beginTransaction().attach(f).commit()
    }
}
`

const fragmentMiscJavaSample = `package com.example;

import android.app.Dialog;
import android.os.Bundle;
import android.view.LayoutInflater;
import androidx.appcompat.app.AlertDialog;
import androidx.fragment.app.DialogFragment;
import androidx.fragment.app.Fragment;
import androidx.fragment.app.FragmentTransaction;
import java.util.Objects;

public class J extends DialogFragment {
    @Override
    public Dialog onCreateDialog(Bundle savedInstanceState) {
        LayoutInflater inflater = LayoutInflater.from(requireContext());
        LayoutInflater other = LayoutInflater.from(getActivity());
        AlertDialog dialog = new AlertDialog.Builder(requireContext()).create();
        dialog.setOnCancelListener(d -> { });
        Objects.requireNonNull(getContext());
        Objects.requireNonNull(getActivity(), "message");
        return dialog;
    }

    void tx(Fragment f) {
        FragmentTransaction t = getParentFragmentManager().beginTransaction();
        t.detach(f);
        t.attach(f);
        t.commit();
    }
}
`

const fragmentDetachMessage = "Calling detach() and attach() in the same FragmentTransaction is a no-op, meaning it does not recreate the Fragment's view. If you would like the view to be recreated, separate these operations into separate transactions."

// Expected positions and messages come from running the lint.jar of androidx.fragment:fragment 1.9.1 through the
// Android Lint CLI on the same files, with the AndroidX classes on the classpath. Lint's null checks are checkNotNull
// and requireNonNull only (requireNotNull(arguments) is not reported), a check with a message argument is not
// rewritten, and a local that shadows the property is skipped.
func TestFragmentMiscLintParity(t *testing.T) {
	kotlin := map[string][]string{
		"UseRequireInsteadOfGet": {
			"14:17 Use requireContext() instead of context!!",
			"15:9 Use requireActivity() instead of activity!!",
			"17:17 Use requireContext() instead of checkNotNull(context)",
			"19:17 Use requireView() instead of view!!",
			"20:17 Use requireParentFragment() instead of parentFragment!!",
			"21:17 Use requireHost() instead of host!!",
			"22:17 Use other.requireContext() instead of other.context!!",
			"23:17 Use this.requireContext() instead of this.context!!",
			"24:17 Use requireContext() instead of getContext()!!",
			"25:17 Use requireContext() instead of (context)!!",
		},
		"UseGetLayoutInflater": {"42:20 Use of LayoutInflater.from(Context) detected. Consider using layoutInflater instead"},
		"DialogFragmentCallbacksDetector": {
			"44:9 Use onCancel() instead of calling setOnCancelListener() from onCreateDialog()",
			"45:9 Use onDismiss() instead of calling setOnDismissListener() from onCreateDialog()",
		},
		"DetachAndAttachSameFragment": {"56:31 " + fragmentDetachMessage, "57:39 " + fragmentDetachMessage},
	}
	java := map[string][]string{
		"UseRequireInsteadOfGet": {"19:9 Use requireContext() instead of Objects.requireNonNull(getContext())"},
		"UseGetLayoutInflater": {
			"15:35 Use of LayoutInflater.from(requireContext()) detected. Consider using getLayoutInflater() instead",
			"16:32 Use of LayoutInflater.from(getActivity()) detected. Consider using getLayoutInflater() instead",
		},
		"DialogFragmentCallbacksDetector": {"18:9 Use onCancel() instead of calling setOnCancelListener() from onCreateDialog()"},
		"DetachAndAttachSameFragment":     {"25:60 " + fragmentDetachMessage},
	}
	for rule, expected := range kotlin {
		t.Run("kotlin/"+rule, func(t *testing.T) {
			sort.Strings(expected)
			if got := fragmentLifecycleFindings(t, rule, "com/example/F.kt", fragmentMiscSample); !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
	for rule, expected := range java {
		t.Run("java/"+rule, func(t *testing.T) {
			sort.Strings(expected)
			if got := fragmentLifecycleFindings(t, rule, "com/example/J.java", fragmentMiscJavaSample); !reflect.DeepEqual(got, expected) {
				t.Fatalf("findings:\n got  %q\n want %q", got, expected)
			}
		})
	}
}

// In an assignment target the not-null assertion is reported like any other; positions from the Lint CLI.
func TestUseRequireInsteadOfGetInAssignmentTargets(t *testing.T) {
	src := `package com.example

import android.view.View
import androidx.fragment.app.Fragment

class F : Fragment() {
    fun hide(child: Fragment) {
        child.view!!.visibility = View.GONE
        view!!.alpha = 0f
    }
}
`
	got := fragmentLifecycleFindings(t, "UseRequireInsteadOfGet", "com/example/F.kt", src)
	want := []string{"8:9 Use child.requireView() instead of child.view!!", "9:9 Use requireView() instead of view!!"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Lint skips only local variables that shadow the getter, so it reports a parameter named context (7:39 in the Lint
// CLI); that parameter is not Fragment.getContext(), and krit does not report it. Neither reports context?.x!!.
func TestUseRequireInsteadOfGetDivergences(t *testing.T) {
	src := `package com.example

import android.content.Context
import androidx.fragment.app.Fragment

class F : Fragment() {
    fun shadowed(context: Context?) = context!!.packageName

    fun chained() = context?.packageName!!
}
`
	if got := fragmentLifecycleFindings(t, "UseRequireInsteadOfGet", "com/example/F.kt", src); len(got) != 0 {
		t.Fatalf("got %q, want none", got)
	}
}

// Only a class whose direct superclass is DialogFragment is reported, in Kotlin too: Lint's Kotlin fallback asks
// whether the class extends the unqualified name "DialogFragment", which never matches. Checked with the Lint CLI.
func TestUseGetLayoutInflaterOnlyDirectDialogFragments(t *testing.T) {
	src := `package x

import android.app.Dialog
import android.os.Bundle
import android.view.LayoutInflater
import androidx.appcompat.app.AppCompatDialogFragment
import androidx.fragment.app.DialogFragment

class A : AppCompatDialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog {
        val inflater = LayoutInflater.from(context)
        return super.onCreateDialog(savedInstanceState)
    }
}

open class Base : DialogFragment()

class B : Base() {
    fun f() = LayoutInflater.from(context)
}
`
	if got := fragmentLifecycleFindings(t, "UseGetLayoutInflater", "x/A.kt", src); len(got) != 0 {
		t.Fatalf("got %q, want none", got)
	}
}
