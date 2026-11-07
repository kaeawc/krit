package rules_test

import "testing"

const securityCallsJava = `package com.example;
import android.net.SSLCertificateSocketFactory;
import android.webkit.SslErrorHandler;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.net.http.SslError;
import java.net.InetAddress;
import java.net.Socket;
import java.security.SecureRandom;
import javax.crypto.Cipher;
public class A extends WebViewClient {
  void f(SSLCertificateSocketFactory factory, InetAddress addr, String host) throws Exception {
    SSLCertificateSocketFactory.getInsecure(0, null);
    Socket s = factory.createSocket(addr, 80);
    Socket t = factory.createSocket(host, 80);
    Socket u = factory.createSocket(InetAddress.getByName(host), 80);
    SecureRandom r = SecureRandom.getInstance("SHA1PRNG", "Crypto");
    Cipher c = Cipher.getInstance("AES", "Crypto");
    Cipher d = Cipher.getInstance("AES", "BC");
    Cipher e = Cipher.getInstance("AES");
  }
  @Override public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
    handler.proceed();
  }
}
class B extends WebViewClient {
  @Override public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
    handler.cancel();
  }
}
class C extends WebViewClient {
  @Override public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
    if (view != null) { handler.proceed(); }
  }
}
`

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestSecurityCallsJavaLintParity(t *testing.T) {
	want := map[string]string{
		"SSLCertificateSocketFactoryGetInsecure":  "13:5",
		"SSLCertificateSocketFactoryCreateSocket": "14:16,16:16",
		"DeletedProvider":                         "17:59,18:42",
		"WebViewClientOnReceivedSslError":         "23:5,33:25",
	}
	for rule, positions := range want {
		t.Run(rule, func(t *testing.T) {
			if got := findingPositions(runRuleByNameOnJava(t, rule, securityCallsJava)); got != positions {
				t.Fatalf("positions = %s, want %s", got, positions)
			}
		})
	}
}

func TestSecurityCallsKotlinLintParity(t *testing.T) {
	src := `package com.example
import android.net.SSLCertificateSocketFactory
import android.net.http.SslError
import android.webkit.SslErrorHandler
import android.webkit.WebView
import android.webkit.WebViewClient
import java.net.InetAddress
import javax.crypto.Cipher
class D : WebViewClient() {
  fun f(factory: SSLCertificateSocketFactory, addr: InetAddress) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    factory.createSocket(addr, 80)
    factory.createSocket("host", 80)
    Cipher.getInstance("AES", "Crypto")
  }
  override fun onReceivedSslError(view: WebView?, handler: SslErrorHandler, error: SslError?) {
    handler.proceed()
  }
}
`
	want := map[string]string{
		"SSLCertificateSocketFactoryGetInsecure":  "11:5",
		"SSLCertificateSocketFactoryCreateSocket": "12:5",
		"DeletedProvider":                         "14:31",
		"WebViewClientOnReceivedSslError":         "17:5",
	}
	for rule, positions := range want {
		t.Run(rule, func(t *testing.T) {
			if got := findingPositions(runRuleByName(t, rule, src)); got != positions {
				t.Fatalf("positions = %s, want %s", got, positions)
			}
		})
	}
}
