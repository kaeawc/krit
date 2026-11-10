package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestSdCardPathKotlinLintParityLiterals(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SdCardPath", `package p
class K {
  fun f() {
    val s0 = "/sdcard/foo"
    val s1 = "/sdcard"
    val s2 = "/sdcardx"
    val s3 = "/mnt/sdcard/x"
    val s4 = "/mnt/sdcard"
    val s5 = "/data/data/com.x"
    val s6 = "/data/data"
    val s7 = "/data/user/0/x"
    val s8 = "file:///sdcard/x"
    val s9 = "file:///data/data/x"
    val s10 = "see /sdcard/ in docs"
    val s11 = "/storage/emulated/0"
    val s12 = "/storage/sdcard0/x"
    val s13 = "/mnt/sdcard0"
    val s14 = "/data/data/"
    val s15 = "/data/user/"
    val s16 = "/sdcard/"
    val s17 = "x/sdcard/y"
    val s18 = "\"/sdcard\""
    val s19 = "/SDCARD/x"
    val s20 = "/mnt/shell/emulated/0"
    val s21 = "/sdcard/a"
    val raw = """/sdcard/raw"""
    val t = "/sdcard/$k"
  }
}
`))
	if want := "4:15,5:15,6:15,7:15,9:15,11:15,12:15,18:16,19:16,20:16,22:18,25:16,26:18,27:14"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSdCardPathJavaLintParityLiterals(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SdCardPath", `package p;
public class J {
  void f() {
    String s0 = "/sdcard/foo";
    String s1 = "/sdcard";
    String s2 = "/sdcardx";
    String s3 = "/mnt/sdcard/x";
    String s4 = "/mnt/sdcard";
    String s5 = "/data/data/com.x";
    String s6 = "/data/data";
    String s7 = "/data/user/0/x";
    String s8 = "file:///sdcard/x";
    String s9 = "file:///data/data/x";
    String s10 = "see /sdcard/ in docs";
    String s11 = "/storage/emulated/0";
    String s12 = "/storage/sdcard0/x";
    String s13 = "/mnt/sdcard0";
    String s14 = "/data/data/";
    String s15 = "/data/user/";
    String s16 = "/sdcard/";
    String s17 = "x/sdcard/y";
    String s18 = "\"/sdcard\"";
    String s19 = "/SDCARD/x";
    String s20 = "/mnt/shell/emulated/0";
    String s21 = "/sdcard/a";
    String raw = "/sdcard/" + "raw";
    String t = "/sdcard/" + k;
  }
}
`))
	if want := "4:17,5:17,6:17,7:17,9:17,11:17,12:17,18:18,19:18,20:18,25:18,26:18,27:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSdCardPathKotlinLintParityTemplates(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SdCardPath", `package p

class K(val k: String) {
  fun f() {
    val a = "$k/sdcard/x"
    val b = "${k}/sdcard/"
    val c = "a ${k} /sdcard/"
    val d = "/sdcard$k"
    val e = "x$k" + "/sdcard/y"
    val g = "file://$k/sdcard"
    val h = "/data/data/$k"
    val i = "/mnt/sdcard/$k/x"
    val j = "/mnt/sdcard"
    val l = "/data/user/${k}"
    val m = "file:///data/user/0"
    val n = "file:///mnt/sdcard/x"
    val o = "/mnt/sdcard0/x"
    val q = "\"/sdcard\""
    val r = "\\/sdcard"
    val s = "\t/sdcard"
    val t = "/sdcard/x"
  }
}
`))
	if want := "5:16,6:18,8:14,9:22,10:23,11:14,12:14,14:14,18:16,19:16,20:16,21:14"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSdCardPathJavaLintParityTemplates(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SdCardPath", `package p;

public class J {
  void f(String k) {
    String a = k + "/sdcard/x";
    String b = "a" + k + " /sdcard/";
    String c = "/sdcard" + k;
    String d = "/data/data/" + k;
    String e = "file://" + k + "/sdcard";
    String f = "\"/sdcard\"";
    String g = "\\/sdcard";
    String h = "\t/sdcard";
    String i = """
      /sdcard/x
      """;
    String j = "/sdcard/x";
  }
}
`))
	if want := "5:20,7:16,8:16,9:32,13:16,16:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
