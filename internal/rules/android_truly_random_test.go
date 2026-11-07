package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// runTrulyRandom runs TrulyRandom over files (name to source) with the given Gradle minSdk.
func runTrulyRandom(t *testing.T, minSdk int, files map[string]string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	var parsed []*scanner.File
	for name, code := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		var file *scanner.File
		var err error
		if filepath.Ext(name) == ".java" {
			file, err = scanner.ParseJavaFile(context.Background(), path)
		} else {
			file, err = scanner.ParseFile(context.Background(), path)
		}
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "TrulyRandom" {
			rule = r
		}
	}
	if rule == nil {
		t.Fatal("TrulyRandom not registered")
	}
	facts := librarymodel.DefaultFacts()
	facts.Profile.MinSdkVersion = minSdk
	collector := scanner.NewFindingCollector(0)
	rule.Check(&api.Context{Rule: rule, ParsedFiles: parsed, Collector: collector, LibraryFacts: facts, DefaultConfidence: 0.9})
	return collector.Columns().Findings()
}

const trulyRandomLintMessage = "Potentially insecure random numbers on Android 4.3 and older. " +
	"Read https://android-developers.blogspot.com/2013/08/some-securerandom-thoughts.html for more info."

func TestTrulyRandom_FollowsLint(t *testing.T) {
	javaCrypto := `package test;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import java.security.SecureRandom;
public class B {
  void a() throws Exception {
    Cipher c = Cipher.getInstance("AES");
    KeyGenerator kg = KeyGenerator.getInstance("AES");
  }
  void b() throws Exception {
    SecureRandom r = new SecureRandom();
  }
}
`
	t.Run("reports once at the first call with Lint's message", func(t *testing.T) {
		findings := runTrulyRandom(t, 16, map[string]string{"B.java": javaCrypto})
		if len(findings) != 1 || findings[0].Line != 8 || findings[0].Message != trulyRandomLintMessage {
			t.Fatalf("expected one finding on line 8, got %+v", findings)
		}
	})
	t.Run("silent when minSdk is above 18", func(t *testing.T) {
		if findings := runTrulyRandom(t, 19, map[string]string{"B.java": javaCrypto}); len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
		if findings := runTrulyRandom(t, 18, map[string]string{"B.java": javaCrypto}); len(findings) != 1 {
			t.Fatalf("expected a finding at minSdk 18, got %+v", findings)
		}
	})
	t.Run("unknown minSdk counts as the lowest", func(t *testing.T) {
		if findings := runTrulyRandom(t, 0, map[string]string{"B.java": javaCrypto}); len(findings) != 1 {
			t.Fatalf("expected a finding, got %+v", findings)
		}
	})
	t.Run("NativeCrypto workaround silences the project", func(t *testing.T) {
		findings := runTrulyRandom(t, 16, map[string]string{
			"B.java": javaCrypto,
			"Fix.java": `package test;
public class Fix { static { try { Class.forName("org.apache.harmony.xnet.provider.jsse.NativeCrypto"); } catch (Exception e) {} } }
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})
	t.Run("Cipher.init only counts for encrypt and wrap modes", func(t *testing.T) {
		src := func(mode string) map[string]string {
			return map[string]string{"C.java": "package test;\nimport javax.crypto.Cipher;\npublic class C {\n  void a(Cipher c, java.security.Key k) throws Exception {\n    c.init(" + mode + ", k);\n  }\n}\n"}
		}
		for mode, want := range map[string]int{"Cipher.ENCRYPT_MODE": 1, "Cipher.WRAP_MODE": 1, "3": 1, "Cipher.DECRYPT_MODE": 0, "2": 0} {
			if got := len(runTrulyRandom(t, 16, src(mode))); got != want {
				t.Errorf("mode %s: expected %d findings, got %d", mode, want, got)
			}
		}
	})
	t.Run("Signature.initSign never fires, as in Lint", func(t *testing.T) {
		findings := runTrulyRandom(t, 16, map[string]string{"S.java": `package test;
import java.security.Signature;
public class S { void a(java.security.PrivateKey k) throws Exception { Signature s = Signature.getInstance("SHA1"); s.initSign(k); } }
`})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})
	t.Run("SecureRandom.getInstance and SSLEngine.wrap", func(t *testing.T) {
		if got := len(runTrulyRandom(t, 16, map[string]string{"R.java": `package test;
import java.security.SecureRandom;
public class R { void a() throws Exception { SecureRandom r = SecureRandom.getInstance("SHA1PRNG"); } }
`})); got != 1 {
			t.Errorf("SecureRandom.getInstance: expected 1 finding, got %d", got)
		}
		if got := len(runTrulyRandom(t, 16, map[string]string{"E.java": `package test;
import javax.net.ssl.SSLEngine;
public class E { void a(SSLEngine e, java.nio.ByteBuffer b) throws Exception { e.wrap(b, b); } }
`})); got != 1 {
			t.Errorf("SSLEngine.wrap: expected 1 finding, got %d", got)
		}
	})
	t.Run("Kotlin sources and lookalikes", func(t *testing.T) {
		if got := len(runTrulyRandom(t, 16, map[string]string{"k.kt": "package test\nimport javax.crypto.KeyGenerator\nfun f() { KeyGenerator.getInstance(\"AES\") }\n"})); got != 1 {
			t.Errorf("Kotlin getInstance: expected 1 finding, got %d", got)
		}
		if got := len(runTrulyRandom(t, 16, map[string]string{"k.kt": "package test\nimport java.security.SecureRandom\nfun f() = SecureRandom()\n"})); got != 1 {
			t.Errorf("Kotlin constructor: expected 1 finding, got %d", got)
		}
		if got := len(runTrulyRandom(t, 16, map[string]string{"k.kt": "package test\nclass KeyGenerator { companion object { fun getInstance(a: String) = 1 } }\nfun f() { KeyGenerator.getInstance(\"AES\") }\n"})); got != 0 {
			t.Errorf("project KeyGenerator lookalike: expected 0 findings, got %d", got)
		}
	})
	t.Run("test sources are skipped", func(t *testing.T) {
		findings := runTrulyRandom(t, 16, map[string]string{"src/test/java/test/B.java": javaCrypto})
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})
}
