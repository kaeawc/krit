package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestWorldFilePermissionsJavaLintParity(t *testing.T) {
	src := `package com.example;
import java.io.File;
public class A {
  static final boolean T = true;
  void f(File file, boolean b) {
    file.setReadable(true, false);
    file.setReadable(true);
    file.setReadable(true, true);
    file.setReadable(T, false);
    file.setReadable(b, false);
    file.setWritable(true, false);
    file.setWritable(false, false);
    new File("x").setWritable(true, false);
    Other o = new Other();
    o.setReadable(true, false);
    boolean r = file.setExecutable(true, false);
  }
}
class Other { void setReadable(boolean a, boolean b) {} }
`
	if got, want := findingPositions(runRuleByNameOnJava(t, "SetWorldReadable", src)), "6:5,9:5"; got != want {
		t.Errorf("SetWorldReadable positions = %s, want %s", got, want)
	}
	if got, want := findingPositions(runRuleByNameOnJava(t, "SetWorldWritable", src)), "11:5,13:5"; got != want {
		t.Errorf("SetWorldWritable positions = %s, want %s", got, want)
	}
}

func TestWorldFilePermissionsKotlinLintParity(t *testing.T) {
	src := `package com.example
import java.io.File
class B {
  fun f(file: File) {
    file.setReadable(true, false)
    file.setWritable(true, false)
    file.setReadable(true)
  }
}
`
	if got, want := findingPositions(runRuleByName(t, "SetWorldReadable", src)), "5:5"; got != want {
		t.Errorf("SetWorldReadable positions = %s, want %s", got, want)
	}
	if got, want := findingPositions(runRuleByName(t, "SetWorldWritable", src)), "6:5"; got != want {
		t.Errorf("SetWorldWritable positions = %s, want %s", got, want)
	}
}
