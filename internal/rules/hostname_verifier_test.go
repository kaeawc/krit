package rules_test

import "testing"

func TestAllowAllHostnameVerifierApache(t *testing.T) {
	for _, test := range []struct {
		name, source string
		java         bool
		want         int
	}{
		{"Kotlin explicit", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier
import org.apache.http.conn.ssl.SSLSocketFactory
val first = AllowAllHostnameVerifier()
val second = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER`, false, 2},
		{"Kotlin aliases", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier as Unsafe
import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as unsafe
val first = Unsafe()
val second = unsafe`, false, 2},
		{"Kotlin FQN", `val first = org.apache.http.conn.ssl.AllowAllHostnameVerifier()
val second = org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER`, false, 2},
		{"Java explicit", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier;
import org.apache.http.conn.ssl.SSLSocketFactory;
class Demo {
 Object first = new AllowAllHostnameVerifier();
 Object second = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
}`, true, 2},
		{"Java static", `import static org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
class Demo { Object verifier = ALLOW_ALL_HOSTNAME_VERIFIER; }`, true, 1},
		{"Java enum constant", `import static org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
enum Policy { ALLOW_ALL_HOSTNAME_VERIFIER, STRICT }`, true, 0},
		{"Java annotation key", `import static org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
@Cfg(ALLOW_ALL_HOSTNAME_VERIFIER = 1) class Demo {}`, true, 0},
		{"Java annotation value", `import static org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
@Cfg(value = ALLOW_ALL_HOSTNAME_VERIFIER) class Demo {}`, true, 1},
		{"Kotlin imported field reference", `import org.apache.http.conn.ssl.SSLSocketFactory
import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
val ref = SSLSocketFactory::ALLOW_ALL_HOSTNAME_VERIFIER`, false, 1},
		{"Java FQN", `class Demo {
 Object first = new org.apache.http.conn.ssl.AllowAllHostnameVerifier();
 Object second = org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER;
}`, true, 2},
		{"Java constructor reference", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier;
class Demo { java.util.function.Supplier<AllowAllHostnameVerifier> factory = AllowAllHostnameVerifier::new; }`, true, 1},
		{"Kotlin superclass constructors", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier
class Derived : AllowAllHostnameVerifier()
val anonymous = object : AllowAllHostnameVerifier() {}`, false, 2},
		{"Kotlin constructor reference", `import org.apache.http.conn.ssl.AllowAllHostnameVerifier
val factory = ::AllowAllHostnameVerifier`, false, 1},
		{"Kotlin property reference", `import org.apache.http.conn.ssl.SSLSocketFactory
val constant = SSLSocketFactory::ALLOW_ALL_HOSTNAME_VERIFIER`, false, 1},
		{"Kotlin field receiver", `import org.apache.http.conn.ssl.SSLSocketFactory
val description = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER.toString()`, false, 1},
		{"Kotlin imported field receiver", `import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as unsafe
val description = unsafe.toString()`, false, 1},
		{"Java field receiver", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Demo { String description = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER.toString(); }`, true, 1},
		{"unresolved", `val first = AllowAllHostnameVerifier()
val second = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER`, false, 0},
		{"other package", `import other.AllowAllHostnameVerifier
import other.SSLSocketFactory
val first = AllowAllHostnameVerifier()
val second = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER`, false, 0},
		{"comment import", `/*
import org.apache.http.conn.ssl.AllowAllHostnameVerifier
*/
val first = AllowAllHostnameVerifier()
val text = "org.apache.http.conn.ssl.AllowAllHostnameVerifier()"`, false, 0},
		{"shadowed parameter", `import org.apache.http.conn.ssl.SSLSocketFactory
fun example(SSLSocketFactory: Other) = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER`, false, 0},
		{"shadowed local", `import org.apache.http.conn.ssl.SSLSocketFactory
fun example() {
 val SSLSocketFactory = Other()
 return SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
}`, false, 0},
		{"custom verify", `import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
class Custom : HostnameVerifier {
 override fun verify(host: String, session: SSLSession) = true
}
val lambda = HostnameVerifier { _, _ -> true }`, false, 0},
		{"Java wildcard", `import org.apache.http.conn.ssl.*;
class Demo { Object verifier = new AllowAllHostnameVerifier(); }`, true, 0},
		{"Java shadowed field", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Demo { Other SSLSocketFactory; Object verifier = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER; }`, true, 0},
		{"constructor parameter", `import org.apache.http.conn.ssl.SSLSocketFactory
class Holder(val SSLSocketFactory: Other) { fun value() = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER }`, false, 0},
		{"object shadow", `import org.apache.http.conn.ssl.SSLSocketFactory
class Cfg { object SSLSocketFactory { const val ALLOW_ALL_HOSTNAME_VERIFIER = "x" }; val value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER }`, false, 0},
		{"companion shadow", `import org.apache.http.conn.ssl.SSLSocketFactory
class Cfg { companion object SSLSocketFactory { const val ALLOW_ALL_HOSTNAME_VERIFIER = "x" }; val value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER }`, false, 0},
		{"Java enum shadow", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Cfg { enum SSLSocketFactory { ALLOW_ALL_HOSTNAME_VERIFIER } Object value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER; }`, true, 0},
		{"Java record shadow", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Cfg { record SSLSocketFactory() { static Object ALLOW_ALL_HOSTNAME_VERIFIER; } Object value = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER; }`, true, 0},
		{"Java lambda parameter", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Cfg { Object value = consume(SSLSocketFactory -> SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER); }`, true, 0},
		{"Java inferred parameters", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Cfg { Object value = consume((SSLSocketFactory, other) -> SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER); }`, true, 0},
		{"Java catch parameter", `import org.apache.http.conn.ssl.SSLSocketFactory;
class Cfg { void value() { try { work(); } catch (Exception SSLSocketFactory) { consume(SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER); } } }`, true, 0},
		{"Kotlin enum entry", `import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER
enum class VerifierPolicy { ALLOW_ALL_HOSTNAME_VERIFIER, STRICT }`, false, 0},
		{"Kotlin named argument", `import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as verifier
val client = Client.create(verifier = DefaultVerifier())`, false, 0},
		{"Kotlin named argument value", `import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as verifier
val client = Client.create(verifier = verifier)`, false, 1},
		{"Kotlin catch parameter", `import org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER as verifier
fun example() { try { work() } catch (verifier: Exception) { log(verifier) } }`, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			findings := runRuleByName(t, "AllowAllHostnameVerifier", test.source)
			if test.java {
				findings = runRuleByNameOnJava(t, "AllowAllHostnameVerifier", test.source)
			}
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), test.want, findings)
			}
		})
	}
}

func TestBadHostnameVerifierJavaOwnerBoundaries(t *testing.T) {
	for _, declaration := range []string{
		"interface Helper { default boolean verify(String host, SSLSession session) { return true; } }",
		"enum Helper { INSTANCE; public boolean verify(String host, SSLSession session) { return true; } }",
		"record Helper() { public boolean verify(String host, SSLSession session) { return true; } }",
		"class Helper { public boolean verify(String host, SSLSession session) { return true; } }",
	} {
		t.Run(declaration, func(t *testing.T) {
			source := "import javax.net.ssl.*; class Strict implements HostnameVerifier { public boolean verify(String host, SSLSession session) { return host.equals(session.getPeerHost()); } " + declaration + " }"
			if findings := runRuleByNameOnJava(t, "BadHostnameVerifier", source); len(findings) != 0 {
				t.Fatalf("inner verify attributed to outer verifier: %v", findings)
			}
		})
	}
}

func TestBadHostnameVerifierWildcardShadow(t *testing.T) {
	for _, source := range []string{
		"import javax.net.ssl.*; interface HostnameVerifier {} class Demo implements HostnameVerifier { public boolean verify(String host, SSLSession session) { return true; } }",
		"import javax.net.ssl.*; class SSLSession {} class Demo implements HostnameVerifier { public boolean verify(String host, SSLSession session) { return true; } }",
	} {
		if findings := runRuleByNameOnJava(t, "BadHostnameVerifier", source); len(findings) != 0 {
			t.Fatalf("wildcard shadow reported: %v", findings)
		}
	}
}

func TestBadHostnameVerifierMainCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       int
	}{
		{"literal", "override fun verify(host: String, session: SSLSession) = true", 1},
		{"no return", "override fun verify(host: String, session: SSLSession) { true }", 1},
		{"parenthesized", "override fun verify(host: String, session: SSLSession) { return (true) }", 0},
		{"comparison", "override fun verify(host: String, session: SSLSession) = host.isEmpty() == true", 0},
		{"conditional false", "override fun verify(host: String, session: SSLSession): Boolean { if (host.isEmpty()) return false; return true }", 1},
		{"nonlocal false", "override fun verify(host: String, session: SSLSession): Boolean { run { if (host.isEmpty()) return false }; return true }", 1},
		{"additional statement", "override fun verify(host: String, session: SSLSession): Boolean { println(host); return true }", 1},
		{"local declaration", "override fun verify(host: String, session: SSLSession): Boolean { val values = listOf(1); return true }", 1},
		{"throws", "override fun verify(host: String, session: SSLSession): Boolean { if (host.isEmpty()) throw IllegalArgumentException(); return true }", 1},
		{"overload", "fun verify(first: Int, second: Int) = true", 0},
		{"vararg overload", "fun verify(host: String, vararg session: SSLSession) = true", 1},
		{"generic", "fun <Value> verify(host: Value, session: SSLSession) = true", 0},
		{"extension", "fun Any.verify(host: String, session: SSLSession) = true", 1},
		{"local", "fun helper() { fun verify(host: String, session: SSLSession) = true }", 0},
		{"anonymous helper", "val helper = fun() { fun verify(host: String, session: SSLSession) = true }", 0},
		{"companion helper", "companion object { fun verify(host: String, session: SSLSession) = true }", 0},
		{"init helper", "init { fun verify(host: String, session: SSLSession) = true }", 0},
		{"constructor helper", "constructor(value: Int) { fun verify(host: String, session: SSLSession) = true }", 0},
		{"getter helper", "val helper: Boolean get() { fun verify(host: String, session: SSLSession) = true; return false }", 0},
		{"setter helper", "var helper: Boolean = false; set(value) { fun verify(host: String, session: SSLSession) = true }", 0},
		{"nested class", "class Helper { fun verify(host: String, session: SSLSession) = true }", 0},
		{"nested interface", "interface Helper { fun verify(host: String, session: SSLSession) = true }", 0},
		{"nested object", "object Helper { fun verify(host: String, session: SSLSession) = true }", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "import javax.net.ssl.HostnameVerifier\nimport javax.net.ssl.SSLSession\nclass Demo : HostnameVerifier {\n" + test.body + "\n}"
			findings := runRuleByName(t, "BadHostnameVerifier", source)
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d: %v", len(findings), test.want, findings)
			}
			if duplicate := runRuleByName(t, "AllowAllHostnameVerifier", source); len(duplicate) != 0 {
				t.Fatalf("custom verify also reported as Apache use: %v", duplicate)
			}
		})
	}
}

func TestBadHostnameVerifierSamConstructors(t *testing.T) {
	for _, expression := range []string{
		"HostnameVerifier { _, _ -> true }",
		"HostnameVerifier { _, _ -> (true) }",
		"HostnameVerifier { _, _ -> return@HostnameVerifier true }",
		"HostnameVerifier({ _, _ -> true })",
	} {
		t.Run(expression, func(t *testing.T) {
			source := "import javax.net.ssl.HostnameVerifier\nval verifier = " + expression
			if findings := runRuleByName(t, "BadHostnameVerifier", source); len(findings) != 1 {
				t.Fatalf("expected one SAM finding, got %v", findings)
			}
			if findings := runRuleByName(t, "AllowAllHostnameVerifier", source); len(findings) != 0 {
				t.Fatalf("expected no Apache findings, got %v", findings)
			}
		})
	}
}

func TestBadHostnameVerifierSamLookalike(t *testing.T) {
	source := `import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
fun HostnameVerifier(predicate: (String, SSLSession) -> Boolean): HostnameVerifier =
    javax.net.ssl.HostnameVerifier { host, _ -> host == "example.com" }
val verifier = HostnameVerifier(predicate = { _, _ -> true })`
	if findings := runRuleByName(t, "BadHostnameVerifier", source); len(findings) != 0 {
		t.Fatalf("a same-named factory is not a SAM constructor: %v", findings)
	}
}

func TestBadHostnameVerifierLocalBase(t *testing.T) {
	java := `import javax.net.ssl.*;
class Outer { HostnameVerifier make() {
 abstract class Base implements HostnameVerifier {}
 class Impl extends Base {
  public boolean verify(String host, SSLSession session) { return true; }
 }
 return new Impl();
} }`
	if got := findingPositions(runRuleByNameOnJava(t, "BadHostnameVerifier", java)); got != "5:18" {
		t.Fatalf("local Java base positions = %s, want 5:18", got)
	}
	kotlin := `import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
fun make(): HostnameVerifier {
 abstract class Base : HostnameVerifier
 class Impl : Base() {
  override fun verify(host: String, session: SSLSession) = true
 }
 return Impl()
}`
	if got := findingPositions(runRuleByName(t, "BadHostnameVerifier", kotlin)); got != "" {
		t.Fatalf("main's superclass-constructor behavior changed: %s", got)
	}
}

func TestBadHostnameVerifierQualifiedAndAliasedParameters(t *testing.T) {
	for _, sessionType := range []string{"SSLSession", "Session", "Chained"} {
		t.Run(sessionType, func(t *testing.T) {
			source := "import javax.net.ssl.HostnameVerifier\nimport javax.net.ssl.SSLSession\ntypealias Session = SSLSession\ntypealias Chained = Session\nclass Demo : HostnameVerifier {\n override fun verify(host: kotlin.String, session: " + sessionType + ") = true\n}"
			if got := findingPositions(runRuleByName(t, "BadHostnameVerifier", source)); got != "6:15" {
				t.Fatalf("qualified/aliased parameters positions = %s, want 6:15", got)
			}
		})
	}
}

func TestBadHostnameVerifierStrictSam(t *testing.T) {
	for _, body := range []string{
		`if (host != "example.com") return@HostnameVerifier false; true`,
		`if (host.isEmpty()) return@HostnameVerifier host == "example.com"; true`,
	} {
		if findings := runRuleByName(t, "BadHostnameVerifier", "import javax.net.ssl.HostnameVerifier\nval strict = HostnameVerifier { host, _ -> "+body+" }"); len(findings) != 0 {
			t.Fatalf("validating SAM must be silent: %v", findings)
		}
	}
}

func TestBadHostnameVerifierImportedLookalike(t *testing.T) {
	source := `import javax.net.ssl.SSLSession
import other.HostnameVerifier
class Lookalike : HostnameVerifier {
 override fun verify(host: String, session: SSLSession) = true
}
class JdkVerifier : javax.net.ssl.HostnameVerifier {
 override fun verify(host: String, session: SSLSession) = true
}`
	if got := findingPositions(runRuleByName(t, "BadHostnameVerifier", source)); got != "7:15" {
		t.Fatalf("imported lookalike positions = %s, want 7:15", got)
	}
}

func TestBadHostnameVerifierAmbiguousBase(t *testing.T) {
	source := `import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
class FirstScope {
    abstract class Base : HostnameVerifier
}
class SecondScope {
    open class Base
    class Helper : Base() {
        fun verify(host: String, session: SSLSession) = true
    }
}`
	if findings := runRuleByName(t, "BadHostnameVerifier", source); len(findings) != 0 {
		t.Fatalf("ambiguous base names must not establish verifier identity: %v", findings)
	}
}
