package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestSuspiciousImportJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SuspiciousImport", `package com.example;
import android.R;
import android.R.layout;
import android.R.*;
import static android.R.layout.simple_list_item_1;
import static android.R.*;
import static android.R.layout.*;
public class A {
}
`))
	if want := "2:1,4:1,6:1"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSuspiciousImportKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SuspiciousImport", `package com.example
import android.R
import android.R.layout
import android.R as AndroidR
import android.R.*
class B
`))
	if want := "2:1,5:1"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
