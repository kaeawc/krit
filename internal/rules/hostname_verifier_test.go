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

func TestBadHostnameVerifierPrecision(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       int
	}{
		{"literal", "override fun verify(host: String, session: SSLSession) = true", 1},
		{"no return", "override fun verify(host: String, session: SSLSession) { true }", 0},
		{"parenthesized", "override fun verify(host: String, session: SSLSession) { return (true) }", 1},
		{"comparison", "override fun verify(host: String, session: SSLSession) = host.isEmpty() == true", 0},
		{"conditional false", "override fun verify(host: String, session: SSLSession): Boolean { if (host.isEmpty()) return false; return true }", 0},
		{"additional statement", "override fun verify(host: String, session: SSLSession): Boolean { println(host); return true }", 0},
		{"overload", "fun verify(first: Int, second: Int) = true", 0},
		{"vararg overload", "fun verify(host: String, vararg session: SSLSession) = true", 0},
		{"generic", "fun <Value> verify(host: Value, session: SSLSession) = true", 0},
		{"extension", "fun Any.verify(host: String, session: SSLSession) = true", 0},
		{"local", "fun helper() { fun verify(host: String, session: SSLSession) = true }", 0},
		{"companion helper", "companion object { fun verify(host: String, session: SSLSession) = true }", 0},
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
