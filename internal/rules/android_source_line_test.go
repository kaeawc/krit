package rules_test

import (
	"testing"
)

func TestWrongImportRule(t *testing.T) {
	t.Run("flags import android.R", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R
class Foo
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for import android.R")
		}
	})

	t.Run("flags import android.R.drawable", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.drawable
class Foo
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for import android.R.drawable")
		}
	})

	t.Run("ignores import com.example.R", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import com.example.R
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for com.example.R, got %d", len(findings))
		}
	})

	t.Run("ignores file without android.R import", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.os.Bundle
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d", len(findings))
		}
	})
}

func TestGetSignaturesRule(t *testing.T) {
	t.Run("flags GET_SIGNATURES usage", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check() {
        val flags = PackageManager.GET_SIGNATURES
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for GET_SIGNATURES")
		}
	})

	t.Run("ignores GET_SIGNING_CERTIFICATES", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check() {
        val flags = PackageManager.GET_SIGNING_CERTIFICATES
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for GET_SIGNING_CERTIFICATES, got %d", len(findings))
		}
	})

	t.Run("ignores file without either constant", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d", len(findings))
		}
	})
}

func TestLayoutInflationRule(t *testing.T) {
	t.Run("flags inflate with null parent", func(t *testing.T) {
		findings := runRuleByName(t, "LayoutInflation", `package test
class Foo {
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.main, null)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent")
		}
	})

	t.Run("dialog context in another function does not suppress inflate", func(t *testing.T) {
		findings := runRuleByName(t, "LayoutInflation", `package test
class Foo {
    fun buildDialog() {
        AlertDialog.Builder(context).setView(View(context))
    }
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.main, null)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent when dialog context is only in another function")
		}
	})

	t.Run("ignores inflate with proper parent", func(t *testing.T) {
		findings := runRuleByName(t, "LayoutInflation", `package test
class Foo {
    fun onCreateView(inflater: LayoutInflater, container: ViewGroup?): View {
        return inflater.inflate(R.layout.main, container, false)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for inflate with parent, got %d", len(findings))
		}
	})
}
