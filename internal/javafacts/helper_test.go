package javafacts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJavacHelperEmitsReceiverAndClassFacts(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not available")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not available")
	}
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	helper := filepath.Join(repoRoot, "tools", "krit-java-facts", "src", "main", "java", "dev", "krit", "javafacts", "Main.java")
	classes := filepath.Join(t.TempDir(), "classes")
	if err := os.MkdirAll(classes, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(javac, "-d", classes, helper).CombinedOutput(); err != nil {
		t.Fatalf("compile helper: %v\n%s", err, out)
	}
	srcRoot := filepath.Join(t.TempDir(), "src")
	writeFile(t, filepath.Join(srcRoot, "android", "webkit", "WebSettings.java"), `package android.webkit; public class WebSettings { public void setJavaScriptEnabled(boolean value) {} }`)
	writeFile(t, filepath.Join(srcRoot, "android", "webkit", "WebView.java"), `package android.webkit; public class WebView { public WebSettings getSettings() { return new WebSettings(); } }`)
	writeFile(t, filepath.Join(srcRoot, "android", "os", "Handler.java"), `package android.os; public class Handler {}`)
	writeFile(t, filepath.Join(srcRoot, "androidx", "recyclerview", "widget", "RecyclerView.java"), `package androidx.recyclerview.widget; public class RecyclerView { public abstract static class Adapter<VH> {} public static class ViewHolder {} }`)
	target := filepath.Join(srcRoot, "test", "Browser.java")
	writeFile(t, target, `package test;
import android.webkit.WebView;
import android.os.Handler;
import androidx.recyclerview.widget.RecyclerView;
class Browser {
  void setup(WebView webView) {
    webView.getSettings().setJavaScriptEnabled(true);
  }
}
class MyHandler extends Handler {}
class MyAdapter extends RecyclerView.Adapter<RecyclerView.ViewHolder> {}`)
	out := filepath.Join(t.TempDir(), "facts.json")
	cmd := exec.Command(java, "-cp", classes, "dev.krit.javafacts.Main", "--out", out, "--classpath", srcRoot, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run helper: %v\n%s", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var sawWebSettings, sawHandler, sawAdapter bool
	for _, call := range facts.Calls {
		if call.Callee == "setJavaScriptEnabled" && call.ReceiverType == "android.webkit.WebSettings" && call.MethodOwner == "android.webkit.WebSettings" {
			sawWebSettings = true
		}
	}
	for _, class := range facts.Classes {
		if class.QualifiedName == "test.MyHandler" && contains(class.Supertypes, "android.os.Handler") {
			sawHandler = true
		}
		if class.QualifiedName == "test.MyAdapter" && contains(class.Supertypes, "androidx.recyclerview.widget.RecyclerView.Adapter<androidx.recyclerview.widget.RecyclerView.ViewHolder>") {
			sawAdapter = true
		}
	}
	if !sawWebSettings || !sawHandler || !sawAdapter {
		t.Fatalf("missing expected facts: webSettings=%v handler=%v adapter=%v facts=%+v", sawWebSettings, sawHandler, sawAdapter, facts)
	}
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
