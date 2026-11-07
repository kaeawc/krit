package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestX509TrustManagerJavaLintParity(t *testing.T) {
	src := `package com.example;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import javax.net.ssl.X509TrustManager;
public class A {
  class Empty implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) throws CertificateException {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws CertificateException { return; }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  class Good implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) throws CertificateException { throw new CertificateException(); }
    public void checkServerTrusted(X509Certificate[] c, String a) throws CertificateException { System.out.println(); }
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  abstract class Abs implements X509TrustManager {
    public abstract void checkServerTrusted(X509Certificate[] c, String a) throws CertificateException;
  }
  void anon() {
    X509TrustManager tm = new X509TrustManager() {
      public void checkClientTrusted(X509Certificate[] c, String a) {}
      public void checkServerTrusted(X509Certificate[] c, String a) {}
      public X509Certificate[] getAcceptedIssuers() { return null; }
    };
  }
}
class Sub extends javax.net.ssl.X509ExtendedTrustManager {
  public void checkClientTrusted(X509Certificate[] c, String a) {}
  public void checkServerTrusted(X509Certificate[] c, String a, java.net.Socket s) {}
  public X509Certificate[] getAcceptedIssuers() { return null; }
}
`
	if got, want := findingPositions(runRuleByNameOnJava(t, "CustomX509TrustManager", src)), "6:9,11:9,16:18,20:31,27:7"; got != want {
		t.Errorf("CustomX509TrustManager positions = %s, want %s", got, want)
	}
	if got, want := findingPositions(runRuleByNameOnJava(t, "TrustAllX509TrustManager", src)), "7:17,8:17,21:19,22:19,28:15,29:15"; got != want {
		t.Errorf("TrustAllX509TrustManager positions = %s, want %s", got, want)
	}
}

func TestX509TrustManagerKotlinLintParity(t *testing.T) {
	src := `package com.example
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager
class B {
  val tm = object : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) { }
    override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
  }
}
class Trust : X509TrustManager {
  override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) = Unit
  override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) { check(chain.isNotEmpty()) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}
interface Iface : X509TrustManager
`
	if got, want := findingPositions(runRuleByName(t, "CustomX509TrustManager", src)), "5:12,11:7"; got != want {
		t.Errorf("CustomX509TrustManager positions = %s, want %s", got, want)
	}
	if got, want := findingPositions(runRuleByName(t, "TrustAllX509TrustManager", src)), "6:18,7:18"; got != want {
		t.Errorf("TrustAllX509TrustManager positions = %s, want %s", got, want)
	}
}
