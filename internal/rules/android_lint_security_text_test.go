package rules_test

import (
	"strings"
	"testing"
)

// Expected positions come from running Android Lint 32.4.1 on the same sources.

// bidiSource spells the bidirectional controls as {RLO}, {PDF}, {LRI}, {PDI} and {RLE} so the test sources stay
// readable.
func bidiSource(s string) string {
	return strings.NewReplacer("{RLO}", "\u202e", "{PDF}", "\u202c", "{LRI}", "\u2066", "{PDI}", "\u2069",
		"{RLE}", "\u202b").Replace(s)
}

func TestBidiSpoofingLintParityKotlin(t *testing.T) {
	src := bidiSource(`package test.pkg

// plain comment {RLO}unterminated
// balanced {RLO}ok{PDF} comment
/* block {LRI}isolate
   reset by newline */
/** kdoc {RLO} */
class Bidi {
    val a = "x{RLO}y"
    val b = "x{RLO}y{PDF}z"
    val c = "pre ${a} {RLE}mid"
    val d = """raw {RLO}
    next"""
    val e = "\u202E escaped"
    val f = "x{LRI}y{PDI}"
    val g = "{RLO}${a}{PDF}"
    val h = "x{RLO}\ty"
    val i = "\t{RLO}z"
    val j = "x{RLO}\t{PDF}y"
    val k = """ok
{RLO}bad"""
    val l = '{RLO}'
}
`)
	got := findingPositions(runRuleByName(t, "BidiSpoofing", src))
	want := "3:1,9:14,11:22,12:16,16:14,17:14,18:16,19:14,21:1"
	if got != want {
		t.Fatalf("BidiSpoofing positions = %s, want %s", got, want)
	}
}

func TestBidiSpoofingLintParityJava(t *testing.T) {
	src := bidiSource(`package test.pkg;

// java comment {RLO}bad
/* java block {RLO} ok {PDF} */
/** javadoc {RLO} here */
public class JCases {
    String a = "x{RLO}y";
    String b = "fine";
    String c = "x" + "{RLO}" + "{PDF}";
    String d = "x\t{RLO}y";
    char e = '{RLO}';
}
`)
	findings := runRuleByNameOnJava(t, "BidiSpoofing", src)
	if got, want := findingPositions(findings), "3:1,7:16,9:22,10:16"; got != want {
		t.Fatalf("BidiSpoofing positions = %s, want %s", got, want)
	}
	msgs := map[string]bool{}
	for _, f := range findings {
		msgs[f.Message] = true
	}
	for _, want := range []string{"Comment contains misleading Unicode bidirectional text",
		"String contains misleading Unicode bidirectional text"} {
		if !msgs[want] {
			t.Errorf("missing message %q in %v", want, msgs)
		}
	}
}

func TestAuthLeakLintParityKotlin(t *testing.T) {
	src := `package test.pkg

class Auth {
    val u1 = "http://user:pass@example.com"
    val u2 = "prefix https://bob:secret@host.io/path"
    val u3 = "ftp://a:%s@host"
    val u5 = "http://user@example.com"
    val u6 = "http://${a()}:pass@host"
    val u7 = """http://raw:pw@host"""
    val u8 = "http://x:%d@host"
    val e = "http://u:p@h\n"
    val f = "\nhttp://u:p@h"
    val g = "a\$b http://u:p@h"
    val i = "$e http://u:p@h"
    fun a() = "q"
}
`
	findings := runRuleByName(t, "AuthLeak", src)
	// Lint starts the range one character into a Kotlin entry: it steps over an opening quote that is not there.
	if got, want := findingPositions(findings), "4:16,5:23,9:18,10:16,11:15,12:17,13:20,14:18"; got != want {
		t.Fatalf("AuthLeak positions = %s, want %s", got, want)
	}
	if findings[0].Message != "Possible credential leak" {
		t.Errorf("message = %q", findings[0].Message)
	}
}

func TestAuthLeakLintParityJava(t *testing.T) {
	src := `package test.pkg;

class J {
    String u1 = "http://user:pass@example.com";
    String u2 = "a" + "http://joe:pw@h";
    String b = "\thttp://u:p@h";
    String c = "ftp://a:%1$s@host";
    String d = """
        http://u:p@h
        """;
}
`
	if got, want := findingPositions(runRuleByNameOnJava(t, "AuthLeak", src)), "4:18,5:24,6:18,8:17"; got != want {
		t.Fatalf("AuthLeak positions = %s, want %s", got, want)
	}
}

func TestAuthLeakSkipsLongLiterals(t *testing.T) {
	long := "http://user:pass@example.com/" + strings.Repeat("a", 500)
	src := "package test.pkg\n\nval s = \"" + long + "\"\n"
	if got := runRuleByName(t, "AuthLeak", src); len(got) != 0 {
		t.Fatalf("AuthLeak reported a literal over 512 characters: %v", got)
	}
}

func TestSecretInSourceLintParityKotlin(t *testing.T) {
	src := `package test.pkg

import com.google.ai.client.generativeai.GenerativeModel

const val TOP_KEY = "AIzaSyA1234567890123456789012345678901"
class Keys {
    companion object { const val CKEY = "AIzaSyB1234567890123456789012345678901" }
}
class Secret {
    val fieldKey = "AIzaSyC1234567890123456789012345678901"
    var mutableKey = "AIzaSyD1234567890123456789012345678901"
    fun f(param: String) {
        GenerativeModel("gemini", "AIzaSyE1234567890123456789012345678901")
        GenerativeModel(modelName = "gemini", apiKey = "AIzaSyF1234567890123456789012345678901")
        GenerativeModel(apiKey = "AIzaSyG1234567890123456789012345678901", modelName = "m")
        val local = "AIzaSyH1234567890123456789012345678901"
        GenerativeModel("gemini", local)
        var localVar = "AIzaSyI1234567890123456789012345678901"
        localVar = "other"
        GenerativeModel("gemini", localVar)
        GenerativeModel("gemini", fieldKey)
        GenerativeModel("gemini", mutableKey)
        GenerativeModel("gemini", TOP_KEY)
        GenerativeModel("gemini", Keys.CKEY)
        GenerativeModel("gemini", ("AIzaSyJ1234567890123456789012345678901"))
        GenerativeModel("gemini", "AIzaShort")
        GenerativeModel("gemini", "BIzaSyK1234567890123456789012345678901")
        GenerativeModel("gemini", "AIzaSyL1234567890123456789012345678901${fieldKey}")
        GenerativeModel("gemini", "AIza" + "SyM1234567890123456789012345678901")
        GenerativeModel("AIzaSyN1234567890123456789012345678901", "x")
        GenerativeModel("gemini", """AIzaSyO1234567890123456789012345678901""")
        GenerativeModel("gemini", param)
        GenerativeModel("gemini", "AIzaSyP1234567890123456789012345678901\n")
    }
}
`
	findings := runRuleByName(t, "SecretInSource", src)
	want := "13:35,14:56,15:34,17:35,20:35,21:35,22:35,23:35,24:35,25:36,31:35"
	if got := findingPositions(findings); got != want {
		t.Fatalf("SecretInSource positions = %s, want %s", got, want)
	}
	if !strings.HasPrefix(findings[0].Message, "This argument looks like an API key that has come from source code") {
		t.Errorf("message = %q", findings[0].Message)
	}
}

func TestSecretInSourceLintParityJava(t *testing.T) {
	src := `package test.pkg;

import com.google.ai.client.generativeai.GenerativeModel;

public class JCases {
    static final String K = "AIzaSyP1234567890123456789012345678901";
    void f() {
        new GenerativeModel("gemini", "AIzaSyQ1234567890123456789012345678901", "");
        String local = "AIzaSyR1234567890123456789012345678901";
        new GenerativeModel("gemini", local, "");
        new GenerativeModel("gemini", K, "");
        new GenerativeModel("gemini", JKeys.JKEY, "");
    }
}
`
	// JKeys lives in another file; resolving it needs the project's source folders and is a known false negative.
	if got, want := findingPositions(runRuleByNameOnJava(t, "SecretInSource", src)), "8:39,10:39,11:39"; got != want {
		t.Fatalf("SecretInSource positions = %s, want %s", got, want)
	}
}

func TestSecretInSourceLookalikesAndSuppression(t *testing.T) {
	kotlin := `package test.pkg

import android.annotation.SuppressLint
import com.google.ai.client.generativeai.GenerativeModel

class S {
    @SuppressLint("SecretInSource")
    val suppressed = "AIzaSyT1234567890123456789012345678901"
    fun f() {
        GenerativeModel("gemini", suppressed)
    }
}
`
	if got := runRuleByName(t, "SecretInSource", kotlin); len(got) != 0 {
		t.Errorf("SecretInSource reported a reference to a suppressed field: %v", got)
	}
	otherPackage := `package test.pkg

import other.GenerativeModel

fun f() = GenerativeModel("gemini", "AIzaSyU1234567890123456789012345678901")
`
	if got := runRuleByName(t, "SecretInSource", otherPackage); len(got) != 0 {
		t.Errorf("SecretInSource reported another package's GenerativeModel: %v", got)
	}
	nested := `package test.pkg

import com.google.ai.client.generativeai.GenerativeModel

class Lookalike {
    class GenerativeModel(val name: String, val apiKey: String)

    val local = GenerativeModel("gemini-pro", "AIzaSyG0000000000000000000000000000000")
}
`
	if got := runRuleByName(t, "SecretInSource", nested); len(got) != 0 {
		t.Errorf("SecretInSource reported a nested class that shadows the import: %v", got)
	}
	java := `package test.pkg;

class JL {
    static class GenerativeModel { GenerativeModel(String a, String b) {} }
    void f() {
        new GenerativeModel("gemini", "AIzaSyW1234567890123456789012345678901");
        new com.google.ai.client.generativeai.GenerativeModel("gemini", "AIzaSyX1234567890123456789012345678901", "");
    }
}
`
	if got, want := findingPositions(runRuleByNameOnJava(t, "SecretInSource", java)), "7:73"; got != want {
		t.Errorf("SecretInSource Java lookalike positions = %s, want %s", got, want)
	}
}
