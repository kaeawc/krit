package rules

import (
	"path/filepath"
	"testing"
)

func TestAncestorDirs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{".", 0},
		{"/a", 2},
		{"/a/b", 3},
	}
	for _, c := range cases {
		got := ancestorDirs(c.in)
		if len(got) != c.want {
			t.Errorf("ancestorDirs(%q) returned %d entries, want %d (%v)", c.in, len(got), c.want, got)
		}
	}
	dirs := ancestorDirs(filepath.Join("/x", "y", "z"))
	if len(dirs) == 0 || dirs[0] != filepath.Clean("/x/y/z") {
		t.Errorf("ancestorDirs leading entry mismatch: %v", dirs)
	}
}

func TestHardcodedHTTPURLInsecure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want bool
	}{
		{"http://example.com", true},
		{"http://api.acme.io/v1", true},
		{"HTTP://Example.com/path", true},
		{"https://example.com", false},
		{"http://localhost:8080/", false},
		{"http://127.0.0.1/health", false},
		{"http://10.0.2.2/", false},
		{"http://0.0.0.0/", false},
		{"ftp://example.com", false},
		{"not a url", false},
		{"", false},
	}
	for _, c := range cases {
		if got := hardcodedHTTPURLInsecure(c.raw); got != c.want {
			t.Errorf("hardcodedHTTPURLInsecure(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestWeakKeySizeThreshold(t *testing.T) {
	t.Parallel()
	cases := []struct {
		algo string
		want int
		ok   bool
	}{
		{"RSA", 2048, true},
		{"DSA", 2048, true},
		{"rsa", 2048, true},
		{"EC", 224, true},
		{"ECDSA", 224, true},
		{"AES", 128, true},
		{"HmacSHA256", 256, true},
		{"hmac-sha512", 256, true},
		{"unknown", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := weakKeySizeThreshold(c.algo)
		if got != c.want || ok != c.ok {
			t.Errorf("weakKeySizeThreshold(%q) = (%d, %v), want (%d, %v)", c.algo, got, ok, c.want, c.ok)
		}
	}
}

func TestRsaNoPaddingAlgorithm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		algo string
		want bool
	}{
		{"RSA/ECB/NoPadding", true},
		{"rsa/ecb/nopadding", true},
		{"RSA/ECB/PKCS1Padding", false},
		{"RSA/ECB/OAEPWithSHA-256AndMGF1Padding", false},
		{"AES/ECB/NoPadding", false},
		{"RSA", false},
		{"", false},
	}
	for _, c := range cases {
		if got := rsaNoPaddingAlgorithm(c.algo); got != c.want {
			t.Errorf("rsaNoPaddingAlgorithm(%q) = %v, want %v", c.algo, got, c.want)
		}
	}
}

func TestPrngFromSystemTimeSeedExpr(t *testing.T) {
	t.Parallel()
	positives := []string{
		"System.currentTimeMillis()",
		"System . currentTimeMillis()",
		"System.nanoTime()",
		"Date().time",
		"Date().getTime()",
		"new Date().getTime()",
		"Calendar.getInstance().timeInMillis",
		"Calendar.getInstance().getTimeInMillis()",
		"Instant.now().toEpochMilli()",
	}
	for _, in := range positives {
		if !prngFromSystemTimeSeedExpr(in) {
			t.Errorf("prngFromSystemTimeSeedExpr(%q) = false, want true", in)
		}
	}
	negatives := []string{
		"42L",
		"someValue",
		"random.nextLong()",
		"",
	}
	for _, in := range negatives {
		if prngFromSystemTimeSeedExpr(in) {
			t.Errorf("prngFromSystemTimeSeedExpr(%q) = true, want false", in)
		}
	}
}

func TestPrngFromSystemTimeTestPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		{"app/src/test/java/Foo.kt", true},
		{"app/src/androidTest/java/Foo.kt", true},
		{"app/src/main/java/Foo.kt", false},
		{"tests/fixtures/whatever", false},
		{"", false},
	}
	for _, c := range cases {
		if got := prngFromSystemTimeTestPath(c.path); got != c.want {
			t.Errorf("prngFromSystemTimeTestPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestStaticIvLiteralKotlinByteArray(t *testing.T) {
	t.Parallel()
	positives := []string{
		"byteArrayOf(0, 1, 2)",
		"byteArrayOf(0x00, 0xff)",
		"kotlin.byteArrayOf(1, 2, 3)",
		"byteArrayOf(1u, 2L)",
		"byteArrayOf('a', 'b')",
	}
	for _, in := range positives {
		if !staticIvLiteralKotlinByteArray(in) {
			t.Errorf("staticIvLiteralKotlinByteArray(%q) = false, want true", in)
		}
	}
	negatives := []string{
		"byteArrayOf(*data)",
		"byteArrayOf(getBytes())",
		"intArrayOf(1, 2)",
		"",
	}
	for _, in := range negatives {
		if staticIvLiteralKotlinByteArray(in) {
			t.Errorf("staticIvLiteralKotlinByteArray(%q) = true, want false", in)
		}
	}
}

func TestStaticIvLiteralJavaByteArray(t *testing.T) {
	t.Parallel()
	if !staticIvLiteralJavaByteArray("new byte[] { 1, 2, 3 }") {
		t.Error("expected positive for new byte[] { 1, 2, 3 }")
	}
	if !staticIvLiteralJavaByteArray("new byte[]{0x01, (byte)0xff}") {
		t.Error("expected positive with hex literal")
	}
	if staticIvLiteralJavaByteArray("new byte[] { foo }") {
		t.Error("expected negative for non-numeric")
	}
	if staticIvLiteralJavaByteArray("new int[] { 1 }") {
		t.Error("expected negative for non-byte")
	}
}

func TestStaticIvLiteralStringBytes(t *testing.T) {
	t.Parallel()
	positives := []string{
		`"hello".toByteArray()`,
		`"hello".encodeToByteArray()`,
		`"hello".getBytes()`,
		`"hello".bytes`,
	}
	for _, in := range positives {
		if !staticIvLiteralStringBytes(in) {
			t.Errorf("staticIvLiteralStringBytes(%q) = false, want true", in)
		}
	}
	if staticIvLiteralStringBytes("foo.toByteArray()") {
		t.Error("expected negative for non-literal receiver")
	}
}

func TestStaticIvLiteralDecodeBytes(t *testing.T) {
	t.Parallel()
	positives := []string{
		`Base64.decode("abc==", 0)`,
		`Base64.getDecoder().decode("abc==")`,
		`"deadbeef".hexToByteArray()`,
	}
	for _, in := range positives {
		if !staticIvLiteralDecodeBytes(in) {
			t.Errorf("staticIvLiteralDecodeBytes(%q) = false, want true", in)
		}
	}
	if staticIvLiteralDecodeBytes("Base64.decode(input, 0)") {
		t.Error("expected negative when input is not a literal")
	}
}

func TestOkHTTPDisableSslValidationAlwaysTrueVerifier(t *testing.T) {
	t.Parallel()
	positives := []string{
		"hostnameVerifier { _, _ -> true }",
		"public boolean verify(...) { return true; }",
		"hostnameVerifier(HostnameVerifier { _, _ -> true })",
	}
	for _, in := range positives {
		if !okHTTPDisableSslValidationAlwaysTrueVerifier(in) {
			t.Errorf("okHTTPDisableSslValidationAlwaysTrueVerifier(%q) = false, want true", in)
		}
	}
	if okHTTPDisableSslValidationAlwaysTrueVerifier("verify { hostname == expected }") {
		t.Error("expected negative for real verifier")
	}
}

func TestOkHTTPDisableSslValidationUnsafeTrustManager(t *testing.T) {
	t.Parallel()
	positives := []string{
		"sslSocketFactory(unsafeFactory)",
		"trustAllManager",
		"X509TrustManager { override fun checkServerTrusted(...) {} }",
	}
	for _, in := range positives {
		if !okHTTPDisableSslValidationUnsafeTrustManager(in) {
			t.Errorf("okHTTPDisableSslValidationUnsafeTrustManager(%q) = false, want true", in)
		}
	}
	if okHTTPDisableSslValidationUnsafeTrustManager("normalSslSocketFactory()") {
		t.Error("expected negative for normal factory")
	}
}

func TestAllowAllHostnameVerifierParamCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		want int
	}{
		{"public boolean verify(String host, SSLSession session) { return true; }", 2},
		{"boolean verify() { return true; }", 0},
		{"verify(a, b, c)", 3},
		{"verify(Map<String, Integer> m, SSLSession s)", 2}, // generic comma must not count
		{"no verify here", -1},
	}
	for _, c := range cases {
		if got := allowAllHostnameVerifierParamCount(c.text); got != c.want {
			t.Errorf("allowAllHostnameVerifierParamCount(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}

func TestAllowAllHostnameVerifierMethodReturnsTrue(t *testing.T) {
	t.Parallel()
	positives := []string{
		"verify(String h, SSLSession s) { return true; }",
		"verify(String h, SSLSession s) { return true }",
		"verify(...) = true",
	}
	for _, in := range positives {
		if !allowAllHostnameVerifierMethodReturnsTrue(in) {
			t.Errorf("allowAllHostnameVerifierMethodReturnsTrue(%q) = false, want true", in)
		}
	}
	negatives := []string{
		"verify(String h, SSLSession s) { return s.isValid(); }",
		"verify(...) = s.isValid()",
		"",
	}
	for _, in := range negatives {
		if allowAllHostnameVerifierMethodReturnsTrue(in) {
			t.Errorf("allowAllHostnameVerifierMethodReturnsTrue(%q) = true, want false", in)
		}
	}
}

func TestMatchingParenIndex(t *testing.T) {
	t.Parallel()
	if got := matchingParenIndex("foo(bar(baz))end", 3); got != 12 {
		t.Errorf("matchingParenIndex outer = %d, want 12", got)
	}
	if got := matchingParenIndex("foo(bar(baz))end", 7); got != 11 {
		t.Errorf("matchingParenIndex inner = %d, want 11", got)
	}
	if got := matchingParenIndex("no parens", 0); got != -1 {
		t.Errorf("matchingParenIndex non-paren = %d, want -1", got)
	}
	if got := matchingParenIndex("foo(unbalanced", 3); got != -1 {
		t.Errorf("matchingParenIndex unbalanced = %d, want -1", got)
	}
}

func TestFirstBraceBody(t *testing.T) {
	t.Parallel()
	body, ok := firstBraceBody("fun foo() { return 1 } trailing")
	if !ok || body != " return 1 " {
		t.Errorf("firstBraceBody simple = (%q, %v)", body, ok)
	}
	body, ok = firstBraceBody("fun a() { if (x) { y() } else { z() } }")
	if !ok || body != " if (x) { y() } else { z() } " {
		t.Errorf("firstBraceBody nested = (%q, %v)", body, ok)
	}
	if _, ok := firstBraceBody("no body"); ok {
		t.Error("firstBraceBody no-brace expected false")
	}
}

func TestStripLineAndBlockComments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"a // comment\nb", "a \nb"},
		{"a /* block */ c", "a  c"},
		{"a /* unterminated", "a "},
		{"plain text", "plain text"},
	}
	for _, c := range cases {
		if got := stripLineAndBlockComments(c.in); got != c.want {
			t.Errorf("stripLineAndBlockComments(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInsecureTrustManagerMethodBodyTrivial(t *testing.T) {
	t.Parallel()
	positives := []struct {
		text, name string
	}{
		{"checkServerTrusted(certs, type) {}", "checkServerTrusted"},
		{"checkClientTrusted(certs, type) { return; }", "checkClientTrusted"},
		{"checkServerTrusted(...) { return }", "checkServerTrusted"},
		{"checkServerTrusted(...) { /* ignored */ }", "checkServerTrusted"},
	}
	for _, c := range positives {
		if !insecureTrustManagerMethodBodyTrivial(c.text, c.name) {
			t.Errorf("expected trivial body for %q", c.text)
		}
	}
	if insecureTrustManagerMethodBodyTrivial("checkServerTrusted(certs, type) { validate(certs); }", "checkServerTrusted") {
		t.Error("expected non-trivial body")
	}
}

func TestInsecureTrustManagerTextHasTypeToken(t *testing.T) {
	t.Parallel()
	if !insecureTrustManagerTextHasTypeToken("class Foo : X509TrustManager {", "X509TrustManager") {
		t.Error("expected match for whole-word token")
	}
	if !insecureTrustManagerTextHasTypeToken("X509TrustManager", "X509TrustManager") {
		t.Error("expected match for bare token")
	}
	if insecureTrustManagerTextHasTypeToken("MyX509TrustManager", "X509TrustManager") {
		t.Error("expected no match when token is a substring of identifier")
	}
	if insecureTrustManagerTextHasTypeToken("X509TrustManagerExt", "X509TrustManager") {
		t.Error("expected no match when token is a prefix of identifier")
	}
}

func TestBroadcastReceiverFlagTextContainsExportedConstant(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"Context.RECEIVER_EXPORTED", true},
		{"Context.RECEIVER_NOT_EXPORTED", true},
		{"0", false},
		{"Context.RECEIVER_VISIBLE_TO_INSTANT_APPS", false},
	}
	for _, c := range cases {
		if got := broadcastReceiverFlagTextContainsExportedConstant(c.in); got != c.want {
			t.Errorf("broadcastReceiverFlagTextContainsExportedConstant(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestStaticIvLiteralList(t *testing.T) {
	t.Parallel()
	positives := []string{
		"1, 2, 3",
		"0x01, 0xff",
		"'a', 'b'",
		"1L, 2u, 0xFFU",
		"1_000, 2_000",
	}
	for _, in := range positives {
		if !staticIvLiteralList(in) {
			t.Errorf("staticIvLiteralList(%q) = false, want true", in)
		}
	}
	negatives := []string{
		"",
		"foo, bar",
		"1, ",
	}
	for _, in := range negatives {
		if staticIvLiteralList(in) {
			t.Errorf("staticIvLiteralList(%q) = true, want false", in)
		}
	}
}
