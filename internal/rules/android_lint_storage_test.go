package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestUsableSpaceJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "UsableSpace", `package com.example;
import java.io.File;
import android.content.Context;
public class A {
  File fld;
  long f(File p, Context c) {
    long a = p.getUsableSpace();
    long b = fld.getUsableSpace();
    long d = c.getFilesDir().getUsableSpace();
    long e = new File("x").getUsableSpace();
    long g = p.getFreeSpace();
    return a + b + d + e + g;
  }
  long h(Other o) { return o.getUsableSpace(); }
}
class Other { long getUsableSpace() { return 0; } }
`))
	if want := "7:16,8:18,9:30,10:28"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUsableSpaceKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UsableSpace", `package com.example
import java.io.File
import android.content.Context
class C(val ctor: File) {
  fun f(p: File, q: File?, c: Context): Long {
    val a = p.usableSpace
    val b = p.getUsableSpace()
    val d = q?.usableSpace
    val e = ctor.usableSpace
    val g = c.filesDir.usableSpace
    val h = File("x").usableSpace
    val i = p.freeSpace
    return a + b + (d ?: 0) + e + g + h + i
  }
  fun other(o: Other) = o.usableSpace
}
class Other { val usableSpace = 1L }
`))
	if want := "6:15,7:15,8:16,9:18,10:24,11:23"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUsableSpaceSkippedWhenAllocatableBytesIsUsed(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "UsableSpace", `package com.example;
import java.io.File;
import android.os.storage.StorageManager;
public class B {
  long f(File p, StorageManager sm) throws Exception {
    sm.getAllocatableBytes(sm.getUuidForPath(p));
    return p.getUsableSpace();
  }
}
`))
	if got != "" {
		t.Fatalf("positions = %s, want none", got)
	}
}
