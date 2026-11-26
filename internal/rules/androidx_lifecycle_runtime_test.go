package rules

import (
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
)

// `x ?: run { … }` proves x non-null only when the run block's last statement exits the function; the statements
// before it (logging, cleanup) do not matter.
func TestNullSafeMutableLiveDataElvisRunExit(t *testing.T) {
	src := `package test

import android.util.Log
import androidx.lifecycle.MutableLiveData

class Model {
    val live = MutableLiveData<String>()

    fun logThenReturn(x: String?) { x ?: run { Log.w("t", "m"); return }; live.value = x }

    fun multiLine(x: String?) {
        x ?: run {
            Log.w("t", "m")
            return
        }
        live.value = x
    }

    fun declared(x: String?) {
        val y = x ?: run {
            println("missing")
            throw IllegalStateException()
        }
        live.value = x
        println(y)
    }

    fun returnOnly(x: String?) {
        x ?: run { return }
        live.value = x
    }

    fun returnLater(): Unit = Unit

    fun callsReturnLater(x: String?) {
        x ?: run { returnLater() }
        live.value = x
    }

    fun returnsFromRun(x: String?) {
        x ?: run { println(); return@run }
        live.value = x
    }

    fun exitsFirst(x: String?) {
        x ?: run {
            if (x == null) println()
        }
        live.value = x
    }
}
`
	got := runAxlRule(t, "NullSafeMutableLiveData", "Model.kt", src, librarymodel.ProjectProfile{})
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	want := []int{37, 42, 49}
	if len(lines) != len(want) {
		t.Fatalf("want findings on lines %v, got %v", want, lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("want findings on lines %v, got %v", want, lines)
		}
	}
}
