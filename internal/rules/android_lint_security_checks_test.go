package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.

func TestHardwareIdsKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "HardwareIds", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "64:5,65:5,69:5,70:5,72:5,73:5,75:22"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUnsafeDynamicallyLoadedCodeKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "UnsafeDynamicallyLoadedCode", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "87:5,88:5,89:5,91:5,94:5,95:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BadHostnameVerifier", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "106:20"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestDeprecatedProviderKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "DeprecatedProvider", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "43:45"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestGetInstanceKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "GetInstance", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "35:24,36:24,37:24,41:24,44:24"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomKLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SecureRandom", `package p

import android.annotation.SuppressLint
import android.content.Context
import android.net.SSLCertificateSocketFactory
import android.provider.Settings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.webkit.SslErrorHandler
import android.net.http.SslError
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K(private val ctx: Context) {
  fun web(w: WebView) {
    w.settings.javaScriptEnabled = true
    w.settings.setJavaScriptEnabled(true)
    w.settings.javaScriptEnabled = false
    w.addJavascriptInterface(Object(), "x")
    w.addJavascriptInterface(Any(), "y")
  }

  @SuppressLint("SetJavaScriptEnabled")
  fun web2(w: WebView) {
    w.settings.javaScriptEnabled = true
  }

  fun cipher() {
    Cipher.getInstance("AES/ECB/NoPadding")
    Cipher.getInstance("DES")
    Cipher.getInstance("AES")
    Cipher.getInstance("AES/CBC/PKCS5Padding")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    val s = "DES"
    Cipher.getInstance(s)
    Cipher.getInstance("des/cbc/pkcs5padding")
    Cipher.getInstance("AES/GCM/NoPadding", "BC")
    Cipher.getInstance("AES/ECB/PKCS5Padding")
  }

  fun files() {
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE)
    ctx.openFileOutput("a", Context.MODE_WORLD_WRITEABLE)
    ctx.getSharedPreferences("a", Context.MODE_WORLD_READABLE)
    ctx.getSharedPreferences("a", Context.MODE_PRIVATE or Context.MODE_WORLD_WRITEABLE)
    ctx.getDir("a", 1)
    ctx.getDir("a", 2)
    val f = File("x")
    f.setReadable(true, false)
    f.setReadable(true)
    f.setWritable(true, false)
    f.setWritable(true, true)
    f.setExecutable(true, false)
    f.setReadable(true, true)
  }

  fun ids() {
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "android_id")
    Settings.System.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
    Settings.Secure.getString(ctx.contentResolver, "other")
    val tm = ctx.getSystemService(Context.TELEPHONY_SERVICE) as android.telephony.TelephonyManager
    tm.deviceId
    tm.getDeviceId()
    tm.imei
    tm.getSubscriberId()
    tm.line1Number
    android.os.Build.getSerial()
    android.os.Build.SERIAL
  }

  fun random() {
    val r = SecureRandom()
    r.setSeed(1L)
    r.setSeed(byteArrayOf(1))
    SecureRandom.getInstance("SHA1PRNG")
    SecureRandom(byteArrayOf(1))
  }

  fun load() {
    System.load("/sdcard/x.so")
    System.load("/data/data/x.so")
    System.load(ctx.filesDir.path + "/x.so")
    System.loadLibrary("x")
    Runtime.getRuntime().load("/sdcard/y.so")
    dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null)
    dalvik.system.PathClassLoader("/sdcard/a.dex", null)
    System.load("/mnt/sdcard/x.so")
    System.load(java.io.File("/sdcard", "x").path)
    Runtime.getRuntime().loadLibrary("x")
  }

  fun ssl(f: SSLCertificateSocketFactory) {
    SSLCertificateSocketFactory.getInsecure(0, null)
    f.createSocket(java.net.InetAddress.getLocalHost(), 1)
    f.createSocket("h", 1)
    f.createSocket(java.net.Socket(), "h", 1, true)
    HttpsURLConnection.setDefaultHostnameVerifier { _, _ -> true }
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean = true
    })
    HttpsURLConnection.setDefaultHostnameVerifier(object : HostnameVerifier {
      override fun verify(h: String, s: SSLSession): Boolean { return h == "x" }
    })
    HttpsURLConnection.setDefaultHostnameVerifier(org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER)
  }
}

class TM : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM2 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { throw java.security.cert.CertificateException() }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}

class TM3 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { println(a) }
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf<X509Certificate>()
}

class TM4 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) { }
  override fun getAcceptedIssuers(): Array<X509Certificate>? = null
}

class WC : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.proceed() }
}
class WC2 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { h.cancel() }
}
class WC3 : WebViewClient() {
  override fun onReceivedSslError(v: WebView, h: SslErrorHandler, e: SslError) { if (e.primaryError == 1) h.proceed() else h.cancel() }
}
`))
	if want := "80:5,81:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "HardwareIds", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "34:5,35:5,37:5,39:5,40:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUnsafeDynamicallyLoadedCodeJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "UnsafeDynamicallyLoadedCode", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "45:5,46:5,48:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "BadHostnameVerifier", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "55:22"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestDeprecatedProviderJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "DeprecatedProvider", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "25:45"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestGetInstanceJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "GetInstance", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "20:24,21:24,22:24"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomJLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SecureRandom", `package p;

import android.content.Context;
import android.provider.Settings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.SslErrorHandler;
import android.net.http.SslError;
import java.io.File;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.net.ssl.*;
import java.security.cert.X509Certificate;

public class J {
  void web(WebView w, Context ctx) throws Exception {
    w.getSettings().setJavaScriptEnabled(true);
    w.getSettings().setJavaScriptEnabled(false);
    w.addJavascriptInterface(new Object(), "x");
    Cipher.getInstance("AES/ECB/NoPadding");
    Cipher.getInstance("DES");
    Cipher.getInstance("AES");
    Cipher.getInstance("AES/CBC/PKCS5Padding");
    Cipher.getInstance("RSA/ECB/PKCS1Padding");
    Cipher.getInstance("AES/GCM/NoPadding", "BC");
    ctx.openFileOutput("a", Context.MODE_WORLD_READABLE);
    ctx.getSharedPreferences("a", Context.MODE_WORLD_WRITEABLE);
    ctx.getDir("a", 2);
    File f = new File("x");
    f.setReadable(true, false);
    f.setReadable(true);
    f.setWritable(true, false);
    f.setExecutable(true, false);
    Settings.Secure.getString(ctx.getContentResolver(), Settings.Secure.ANDROID_ID);
    Settings.Secure.getString(ctx.getContentResolver(), "android_id");
    android.telephony.TelephonyManager tm = (android.telephony.TelephonyManager) ctx.getSystemService(Context.TELEPHONY_SERVICE);
    tm.getDeviceId();
    tm.getImei();
    tm.getSubscriberId();
    tm.getLine1Number();
    SecureRandom r = new SecureRandom();
    r.setSeed(1L);
    SecureRandom.getInstance("SHA1PRNG");
    new SecureRandom(new byte[]{1});
    System.load("/sdcard/x.so");
    System.load("/data/data/x.so");
    System.loadLibrary("x");
    Runtime.getRuntime().load("/sdcard/y.so");
    new dalvik.system.DexClassLoader("/sdcard/a.dex", "o", null, null);
    android.net.SSLCertificateSocketFactory.getInsecure(0, null);
    android.net.SSLCertificateSocketFactory sf = null;
    sf.createSocket("h", 1);
    sf.createSocket(java.net.InetAddress.getLocalHost(), 1);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return true; }
    });
    HttpsURLConnection.setDefaultHostnameVerifier((h, s) -> true);
    HttpsURLConnection.setDefaultHostnameVerifier(new HostnameVerifier() {
      public boolean verify(String h, SSLSession s) { return h.equals("x"); }
    });
  }
  static class TM implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) {}
    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
  }
  static class TM2 implements X509TrustManager {
    public void checkClientTrusted(X509Certificate[] c, String a) {}
    public void checkServerTrusted(X509Certificate[] c, String a) throws java.security.cert.CertificateException { throw new java.security.cert.CertificateException(); }
    public X509Certificate[] getAcceptedIssuers() { return null; }
  }
  static class WC extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { h.proceed(); }
  }
  static class WC3 extends WebViewClient {
    public void onReceivedSslError(WebView v, SslErrorHandler h, SslError e) { if (e != null) h.proceed(); else h.cancel(); }
  }
}
`))
	if want := "42:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsK2LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "HardwareIds", `package p

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.bluetooth.BluetoothAdapter
import android.provider.Settings
import android.telephony.TelephonyManager
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

private const val ALGO = "DES"
private const val ECB = "AES/ECB/PKCS5Padding"

class K2(private val ctx: Context, private val tm: TelephonyManager, private val wm: WifiManager) {
  private val lazyTm: TelephonyManager by lazy { ctx.getSystemService(TelephonyManager::class.java) }

  fun ids(info: WifiInfo, ad: BluetoothAdapter) {
    info.macAddress
    info.getMacAddress()
    ad.address
    ad.getAddress()
    tm.simSerialNumber
    tm.getSimSerialNumber()
    tm.imei
    tm.meid
    tm.deviceId
    tm.getDeviceId(0)
    tm.subscriberId
    lazyTm.line1Number
    wm.connectionInfo.macAddress
    val tm2 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    tm2.line1Number
    val tm3 = ctx.getSystemService(TelephonyManager::class.java)
    tm3.getLine1Number()
    Settings.Secure.getString(ctx.contentResolver, ANDROID)
    Settings.Secure.getInt(ctx.contentResolver, "android_id")
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID + "x")
    try {
      tm.deviceId
    } catch (e: Exception) {
    }
  }

  fun cipher(provider: String) {
    Cipher.getInstance(ALGO)
    Cipher.getInstance(ECB)
    Cipher.getInstance("AES/CBC/ECB")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    Cipher.getInstance("AES/ECB")
    Cipher.getInstance("RSA/None/ECB")
    Cipher.getInstance(provider)
    Cipher.getInstance("AES" + "")
    Cipher.getInstance("DESede")
    Cipher.getInstance("AES/CBC/PKCS5Padding", "BC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", provider)
    Cipher.getInstance("AES/CBC/PKCS5Padding", "SC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", BCN)
    Cipher.getInstance("AES/CBC/PKCS5Padding", java.security.Security.getProvider("BC"))
  }

  fun random() {
    val r = SecureRandom()
    val seed = 42L
    var mseed = 42L
    r.setSeed(seed)
    r.setSeed(mseed)
    r.setSeed(-5L)
    r.setSeed(seed + 1)
    r.setSeed(System.currentTimeMillis())
    r.setSeed(System.nanoTime())
    r.setSeed(intArrayOf(1, 2).size.toLong())
    r.setSeed(byteArrayOf())
    r.setSeed(byteArrayOf(1, 2, 3))
    r.setSeed(r.generateSeed(8))
    r.setSeed(ByteArray(8))
    r.setSeed("abc".toByteArray())
    r.setSeed(0x10L)
    r.setSeed(1.toLong())
    val b = byteArrayOf(9)
    r.setSeed(b)
    r.setSeed(SEED)
    r.setSeed(Long.MAX_VALUE)
  }
}

private const val SEED = 7L
private const val ANDROID = "android_id"
private const val BCN = "BC"

val v1 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = HostnameVerifier { _, _ -> true }

class V9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  override fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  override fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean { return (true) }
}
`))
	if want := "21:5,22:5,23:5,24:5,25:5,26:5,29:5,30:5,31:5,32:5,33:5,35:5,38:5,42:7"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierK2LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BadHostnameVerifier", `package p

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.bluetooth.BluetoothAdapter
import android.provider.Settings
import android.telephony.TelephonyManager
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

private const val ALGO = "DES"
private const val ECB = "AES/ECB/PKCS5Padding"

class K2(private val ctx: Context, private val tm: TelephonyManager, private val wm: WifiManager) {
  private val lazyTm: TelephonyManager by lazy { ctx.getSystemService(TelephonyManager::class.java) }

  fun ids(info: WifiInfo, ad: BluetoothAdapter) {
    info.macAddress
    info.getMacAddress()
    ad.address
    ad.getAddress()
    tm.simSerialNumber
    tm.getSimSerialNumber()
    tm.imei
    tm.meid
    tm.deviceId
    tm.getDeviceId(0)
    tm.subscriberId
    lazyTm.line1Number
    wm.connectionInfo.macAddress
    val tm2 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    tm2.line1Number
    val tm3 = ctx.getSystemService(TelephonyManager::class.java)
    tm3.getLine1Number()
    Settings.Secure.getString(ctx.contentResolver, ANDROID)
    Settings.Secure.getInt(ctx.contentResolver, "android_id")
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID + "x")
    try {
      tm.deviceId
    } catch (e: Exception) {
    }
  }

  fun cipher(provider: String) {
    Cipher.getInstance(ALGO)
    Cipher.getInstance(ECB)
    Cipher.getInstance("AES/CBC/ECB")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    Cipher.getInstance("AES/ECB")
    Cipher.getInstance("RSA/None/ECB")
    Cipher.getInstance(provider)
    Cipher.getInstance("AES" + "")
    Cipher.getInstance("DESede")
    Cipher.getInstance("AES/CBC/PKCS5Padding", "BC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", provider)
    Cipher.getInstance("AES/CBC/PKCS5Padding", "SC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", BCN)
    Cipher.getInstance("AES/CBC/PKCS5Padding", java.security.Security.getProvider("BC"))
  }

  fun random() {
    val r = SecureRandom()
    val seed = 42L
    var mseed = 42L
    r.setSeed(seed)
    r.setSeed(mseed)
    r.setSeed(-5L)
    r.setSeed(seed + 1)
    r.setSeed(System.currentTimeMillis())
    r.setSeed(System.nanoTime())
    r.setSeed(intArrayOf(1, 2).size.toLong())
    r.setSeed(byteArrayOf())
    r.setSeed(byteArrayOf(1, 2, 3))
    r.setSeed(r.generateSeed(8))
    r.setSeed(ByteArray(8))
    r.setSeed("abc".toByteArray())
    r.setSeed(0x10L)
    r.setSeed(1.toLong())
    val b = byteArrayOf(9)
    r.setSeed(b)
    r.setSeed(SEED)
    r.setSeed(Long.MAX_VALUE)
  }
}

private const val SEED = 7L
private const val ANDROID = "android_id"
private const val BCN = "BC"

val v1 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = HostnameVerifier { _, _ -> true }

class V9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  override fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  override fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean { return (true) }
}
`))
	if want := "94:16,99:16,105:16,132:16,139:16,147:16,151:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestDeprecatedProviderK2LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "DeprecatedProvider", `package p

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.bluetooth.BluetoothAdapter
import android.provider.Settings
import android.telephony.TelephonyManager
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

private const val ALGO = "DES"
private const val ECB = "AES/ECB/PKCS5Padding"

class K2(private val ctx: Context, private val tm: TelephonyManager, private val wm: WifiManager) {
  private val lazyTm: TelephonyManager by lazy { ctx.getSystemService(TelephonyManager::class.java) }

  fun ids(info: WifiInfo, ad: BluetoothAdapter) {
    info.macAddress
    info.getMacAddress()
    ad.address
    ad.getAddress()
    tm.simSerialNumber
    tm.getSimSerialNumber()
    tm.imei
    tm.meid
    tm.deviceId
    tm.getDeviceId(0)
    tm.subscriberId
    lazyTm.line1Number
    wm.connectionInfo.macAddress
    val tm2 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    tm2.line1Number
    val tm3 = ctx.getSystemService(TelephonyManager::class.java)
    tm3.getLine1Number()
    Settings.Secure.getString(ctx.contentResolver, ANDROID)
    Settings.Secure.getInt(ctx.contentResolver, "android_id")
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID + "x")
    try {
      tm.deviceId
    } catch (e: Exception) {
    }
  }

  fun cipher(provider: String) {
    Cipher.getInstance(ALGO)
    Cipher.getInstance(ECB)
    Cipher.getInstance("AES/CBC/ECB")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    Cipher.getInstance("AES/ECB")
    Cipher.getInstance("RSA/None/ECB")
    Cipher.getInstance(provider)
    Cipher.getInstance("AES" + "")
    Cipher.getInstance("DESede")
    Cipher.getInstance("AES/CBC/PKCS5Padding", "BC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", provider)
    Cipher.getInstance("AES/CBC/PKCS5Padding", "SC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", BCN)
    Cipher.getInstance("AES/CBC/PKCS5Padding", java.security.Security.getProvider("BC"))
  }

  fun random() {
    val r = SecureRandom()
    val seed = 42L
    var mseed = 42L
    r.setSeed(seed)
    r.setSeed(mseed)
    r.setSeed(-5L)
    r.setSeed(seed + 1)
    r.setSeed(System.currentTimeMillis())
    r.setSeed(System.nanoTime())
    r.setSeed(intArrayOf(1, 2).size.toLong())
    r.setSeed(byteArrayOf())
    r.setSeed(byteArrayOf(1, 2, 3))
    r.setSeed(r.generateSeed(8))
    r.setSeed(ByteArray(8))
    r.setSeed("abc".toByteArray())
    r.setSeed(0x10L)
    r.setSeed(1.toLong())
    val b = byteArrayOf(9)
    r.setSeed(b)
    r.setSeed(SEED)
    r.setSeed(Long.MAX_VALUE)
  }
}

private const val SEED = 7L
private const val ANDROID = "android_id"
private const val BCN = "BC"

val v1 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = HostnameVerifier { _, _ -> true }

class V9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  override fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  override fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean { return (true) }
}
`))
	if want := "57:48,60:48,61:48"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestGetInstanceK2LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "GetInstance", `package p

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.bluetooth.BluetoothAdapter
import android.provider.Settings
import android.telephony.TelephonyManager
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

private const val ALGO = "DES"
private const val ECB = "AES/ECB/PKCS5Padding"

class K2(private val ctx: Context, private val tm: TelephonyManager, private val wm: WifiManager) {
  private val lazyTm: TelephonyManager by lazy { ctx.getSystemService(TelephonyManager::class.java) }

  fun ids(info: WifiInfo, ad: BluetoothAdapter) {
    info.macAddress
    info.getMacAddress()
    ad.address
    ad.getAddress()
    tm.simSerialNumber
    tm.getSimSerialNumber()
    tm.imei
    tm.meid
    tm.deviceId
    tm.getDeviceId(0)
    tm.subscriberId
    lazyTm.line1Number
    wm.connectionInfo.macAddress
    val tm2 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    tm2.line1Number
    val tm3 = ctx.getSystemService(TelephonyManager::class.java)
    tm3.getLine1Number()
    Settings.Secure.getString(ctx.contentResolver, ANDROID)
    Settings.Secure.getInt(ctx.contentResolver, "android_id")
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID + "x")
    try {
      tm.deviceId
    } catch (e: Exception) {
    }
  }

  fun cipher(provider: String) {
    Cipher.getInstance(ALGO)
    Cipher.getInstance(ECB)
    Cipher.getInstance("AES/CBC/ECB")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    Cipher.getInstance("AES/ECB")
    Cipher.getInstance("RSA/None/ECB")
    Cipher.getInstance(provider)
    Cipher.getInstance("AES" + "")
    Cipher.getInstance("DESede")
    Cipher.getInstance("AES/CBC/PKCS5Padding", "BC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", provider)
    Cipher.getInstance("AES/CBC/PKCS5Padding", "SC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", BCN)
    Cipher.getInstance("AES/CBC/PKCS5Padding", java.security.Security.getProvider("BC"))
  }

  fun random() {
    val r = SecureRandom()
    val seed = 42L
    var mseed = 42L
    r.setSeed(seed)
    r.setSeed(mseed)
    r.setSeed(-5L)
    r.setSeed(seed + 1)
    r.setSeed(System.currentTimeMillis())
    r.setSeed(System.nanoTime())
    r.setSeed(intArrayOf(1, 2).size.toLong())
    r.setSeed(byteArrayOf())
    r.setSeed(byteArrayOf(1, 2, 3))
    r.setSeed(r.generateSeed(8))
    r.setSeed(ByteArray(8))
    r.setSeed("abc".toByteArray())
    r.setSeed(0x10L)
    r.setSeed(1.toLong())
    val b = byteArrayOf(9)
    r.setSeed(b)
    r.setSeed(SEED)
    r.setSeed(Long.MAX_VALUE)
  }
}

private const val SEED = 7L
private const val ANDROID = "android_id"
private const val BCN = "BC"

val v1 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = HostnameVerifier { _, _ -> true }

class V9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  override fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  override fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean { return (true) }
}
`))
	if want := "48:24,49:24,50:24,52:24,55:24,56:24"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomK2LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SecureRandom", `package p

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager
import android.bluetooth.BluetoothAdapter
import android.provider.Settings
import android.telephony.TelephonyManager
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

private const val ALGO = "DES"
private const val ECB = "AES/ECB/PKCS5Padding"

class K2(private val ctx: Context, private val tm: TelephonyManager, private val wm: WifiManager) {
  private val lazyTm: TelephonyManager by lazy { ctx.getSystemService(TelephonyManager::class.java) }

  fun ids(info: WifiInfo, ad: BluetoothAdapter) {
    info.macAddress
    info.getMacAddress()
    ad.address
    ad.getAddress()
    tm.simSerialNumber
    tm.getSimSerialNumber()
    tm.imei
    tm.meid
    tm.deviceId
    tm.getDeviceId(0)
    tm.subscriberId
    lazyTm.line1Number
    wm.connectionInfo.macAddress
    val tm2 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    tm2.line1Number
    val tm3 = ctx.getSystemService(TelephonyManager::class.java)
    tm3.getLine1Number()
    Settings.Secure.getString(ctx.contentResolver, ANDROID)
    Settings.Secure.getInt(ctx.contentResolver, "android_id")
    Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID + "x")
    try {
      tm.deviceId
    } catch (e: Exception) {
    }
  }

  fun cipher(provider: String) {
    Cipher.getInstance(ALGO)
    Cipher.getInstance(ECB)
    Cipher.getInstance("AES/CBC/ECB")
    Cipher.getInstance("RSA/ECB/PKCS1Padding")
    Cipher.getInstance("AES/ECB")
    Cipher.getInstance("RSA/None/ECB")
    Cipher.getInstance(provider)
    Cipher.getInstance("AES" + "")
    Cipher.getInstance("DESede")
    Cipher.getInstance("AES/CBC/PKCS5Padding", "BC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", provider)
    Cipher.getInstance("AES/CBC/PKCS5Padding", "SC")
    Cipher.getInstance("AES/CBC/PKCS5Padding", BCN)
    Cipher.getInstance("AES/CBC/PKCS5Padding", java.security.Security.getProvider("BC"))
  }

  fun random() {
    val r = SecureRandom()
    val seed = 42L
    var mseed = 42L
    r.setSeed(seed)
    r.setSeed(mseed)
    r.setSeed(-5L)
    r.setSeed(seed + 1)
    r.setSeed(System.currentTimeMillis())
    r.setSeed(System.nanoTime())
    r.setSeed(intArrayOf(1, 2).size.toLong())
    r.setSeed(byteArrayOf())
    r.setSeed(byteArrayOf(1, 2, 3))
    r.setSeed(r.generateSeed(8))
    r.setSeed(ByteArray(8))
    r.setSeed("abc".toByteArray())
    r.setSeed(0x10L)
    r.setSeed(1.toLong())
    val b = byteArrayOf(9)
    r.setSeed(b)
    r.setSeed(SEED)
    r.setSeed(Long.MAX_VALUE)
  }
}

private const val SEED = 7L
private const val ANDROID = "android_id"
private const val BCN = "BC"

val v1 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
  }
}
val v2 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return false
    return true
  }
}
val v3 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    println(h)
    return true
  }
}
val v4 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h == "a") return true
    return false
  }
}
val v5 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return false
  }
}
val v6 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    throw IllegalStateException()
  }
}
val v7 = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = (h.length > 0)
}
val v8 = HostnameVerifier { _, _ -> true }

class V9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
  fun other() { println("x") }
}
open class V11 : HostnameVerifier {
  override fun verify(h: String?, s: SSLSession?): Boolean = true
}
class V12 : V11()
class V13 : V11() {
  override fun verify(h: String?, s: SSLSession?): Boolean { return true }
}
abstract class V14 : HostnameVerifier
interface V15 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V16 : HostnameVerifier {
  fun verify(h: String): Boolean = true
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class V17 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean { return (true) }
}
`))
	if want := "68:5,69:5,70:5,71:5,72:5,73:5,75:5,76:5,78:5,80:5,81:5,83:5,84:5,85:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsJ2LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "HardwareIds", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
import java.security.SecureRandom;

public class J2 {
  static final long S = 5L;
  void r() {
    SecureRandom r = new SecureRandom();
    r.setSeed(S);
    r.setSeed(new byte[] {1, 2});
    r.setSeed(new byte[8]);
    r.setSeed(-1L);
    r.setSeed(S + 2);
    long x = 3;
    r.setSeed(x);
    r.setSeed(System.nanoTime());
    r.setSeed(r.nextLong());
  }
  static class A implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class B implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { if (h.isEmpty()) { return false; } return true; }
  }
  static class C implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return false; }
    public boolean verify(String h, SSLSession s, int x) { return true; }
  }
  static class D implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return !false; }
  }
  static class E extends A {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class F extends A { }
}
`))
	if want := ""; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierJ2LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "BadHostnameVerifier", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
import java.security.SecureRandom;

public class J2 {
  static final long S = 5L;
  void r() {
    SecureRandom r = new SecureRandom();
    r.setSeed(S);
    r.setSeed(new byte[] {1, 2});
    r.setSeed(new byte[8]);
    r.setSeed(-1L);
    r.setSeed(S + 2);
    long x = 3;
    r.setSeed(x);
    r.setSeed(System.nanoTime());
    r.setSeed(r.nextLong());
  }
  static class A implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class B implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { if (h.isEmpty()) { return false; } return true; }
  }
  static class C implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return false; }
    public boolean verify(String h, SSLSession s, int x) { return true; }
  }
  static class D implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return !false; }
  }
  static class E extends A {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class F extends A { }
}
`))
	if want := "22:20,25:20,28:20,35:20"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestDeprecatedProviderJ2LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "DeprecatedProvider", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
import java.security.SecureRandom;

public class J2 {
  static final long S = 5L;
  void r() {
    SecureRandom r = new SecureRandom();
    r.setSeed(S);
    r.setSeed(new byte[] {1, 2});
    r.setSeed(new byte[8]);
    r.setSeed(-1L);
    r.setSeed(S + 2);
    long x = 3;
    r.setSeed(x);
    r.setSeed(System.nanoTime());
    r.setSeed(r.nextLong());
  }
  static class A implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class B implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { if (h.isEmpty()) { return false; } return true; }
  }
  static class C implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return false; }
    public boolean verify(String h, SSLSession s, int x) { return true; }
  }
  static class D implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return !false; }
  }
  static class E extends A {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class F extends A { }
}
`))
	if want := ""; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestGetInstanceJ2LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "GetInstance", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
import java.security.SecureRandom;

public class J2 {
  static final long S = 5L;
  void r() {
    SecureRandom r = new SecureRandom();
    r.setSeed(S);
    r.setSeed(new byte[] {1, 2});
    r.setSeed(new byte[8]);
    r.setSeed(-1L);
    r.setSeed(S + 2);
    long x = 3;
    r.setSeed(x);
    r.setSeed(System.nanoTime());
    r.setSeed(r.nextLong());
  }
  static class A implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class B implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { if (h.isEmpty()) { return false; } return true; }
  }
  static class C implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return false; }
    public boolean verify(String h, SSLSession s, int x) { return true; }
  }
  static class D implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return !false; }
  }
  static class E extends A {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class F extends A { }
}
`))
	if want := ""; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomJ2LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SecureRandom", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
import java.security.SecureRandom;

public class J2 {
  static final long S = 5L;
  void r() {
    SecureRandom r = new SecureRandom();
    r.setSeed(S);
    r.setSeed(new byte[] {1, 2});
    r.setSeed(new byte[8]);
    r.setSeed(-1L);
    r.setSeed(S + 2);
    long x = 3;
    r.setSeed(x);
    r.setSeed(System.nanoTime());
    r.setSeed(r.nextLong());
  }
  static class A implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class B implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { if (h.isEmpty()) { return false; } return true; }
  }
  static class C implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return false; }
    public boolean verify(String h, SSLSession s, int x) { return true; }
  }
  static class D implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return !false; }
  }
  static class E extends A {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class F extends A { }
}
`))
	if want := "11:5,12:5,13:5,14:5,15:5,17:5,18:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsK3LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "HardwareIds", `package p

import android.content.Context
import android.telephony.TelephonyManager
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K3(private val ctx: Context) {
  fun a() {
    val t1 = ctx.getSystemService(TelephonyManager::class.java)
    t1.line1Number
    val t2: TelephonyManager = ctx.getSystemService(TelephonyManager::class.java)
    t2.line1Number
    val t3 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    t3.line1Number
    val t4 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as? TelephonyManager
    t4?.line1Number
    val t5 = ctx.getSystemService(TelephonyManager::class.java)!!
    t5.line1Number
    (ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager).line1Number
    ctx.getSystemService(TelephonyManager::class.java).line1Number
    ctx.getSystemService(TelephonyManager::class.java)?.line1Number
    val t6 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    t6.getLine1Number()
    t6?.line1Number
  }
}

open class H1 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
open class H2 : H1() {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class H3 : H2() {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
open class T1 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}
class T2 : T1() {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
}
`))
	if want := "15:5,17:5,19:5,22:5,26:5,27:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierK3LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BadHostnameVerifier", `package p

import android.content.Context
import android.telephony.TelephonyManager
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager
import java.security.cert.X509Certificate

class K3(private val ctx: Context) {
  fun a() {
    val t1 = ctx.getSystemService(TelephonyManager::class.java)
    t1.line1Number
    val t2: TelephonyManager = ctx.getSystemService(TelephonyManager::class.java)
    t2.line1Number
    val t3 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    t3.line1Number
    val t4 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as? TelephonyManager
    t4?.line1Number
    val t5 = ctx.getSystemService(TelephonyManager::class.java)!!
    t5.line1Number
    (ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager).line1Number
    ctx.getSystemService(TelephonyManager::class.java).line1Number
    ctx.getSystemService(TelephonyManager::class.java)?.line1Number
    val t6 = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
    t6.getLine1Number()
    t6?.line1Number
  }
}

open class H1 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
open class H2 : H1() {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class H3 : H2() {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
open class T1 : X509TrustManager {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
  override fun checkServerTrusted(c: Array<X509Certificate>, a: String) {}
  override fun getAcceptedIssuers(): Array<X509Certificate> = arrayOf()
}
class T2 : T1() {
  override fun checkClientTrusted(c: Array<X509Certificate>, a: String) {}
}
`))
	if want := "32:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierK4LintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BadHostnameVerifier", `package p

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

open class Base
class P1 : Base(), HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P2(val a: Int) : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P3 : HostnameVerifier {
  constructor()
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P4 : HostnameVerifier {
  init { }
  val x = 1
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P5 : HostnameVerifier {
  val x = compute()
  fun compute() = 1
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P6 : HostnameVerifier by Impl() {
}
class Impl : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = true
}
class P7 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    val l = listOf(1)
    return true
  }
}
class P8 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    if (h.isEmpty()) throw RuntimeException()
    return true
  }
}
class P9 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean = when { true -> true else -> true }
}
class P10 : HostnameVerifier {
  override fun verify(h: String, s: SSLSession): Boolean {
    return true
    return false
  }
}
class P11 : HostnameVerifier {
  companion object { fun f() = println() }
  override fun verify(h: String, s: SSLSession): Boolean = true
}
val lam = HostnameVerifier { h, s -> true }
fun fn(): HostnameVerifier = object : HostnameVerifier {
  override fun verify(h: String, s: SSLSession) = true
}
`))
	if want := "11:16,20:16,25:16,30:16,33:16,39:16,59:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierJ4LintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "BadHostnameVerifier", `package p;
import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;
public class J4 {
  static class Base {}
  static class Q1 extends Base implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class Q2 implements HostnameVerifier {
    Q2() {}
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class Q3 implements HostnameVerifier {
    Q3(int a) { super(); }
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static class Q4 implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { return h != null || true; }
  }
  static class Q5 implements HostnameVerifier {
    public boolean verify(String h, SSLSession s) { boolean b = true; return b; }
  }
  static class Q6 implements HostnameVerifier {
    final boolean ok = Boolean.TRUE;
    public boolean verify(String h, SSLSession s) { return true; }
  }
  static HostnameVerifier a = new HostnameVerifier() { public boolean verify(String h, SSLSession s) { return true; } };
  static HostnameVerifier b = (h, s) -> true;
  static HostnameVerifier c = new Base2() { public boolean verify(String h, SSLSession s) { return true; } };
  static abstract class Base2 implements HostnameVerifier {}
}
`))
	if want := "7:20,11:20,15:20,25:20,27:71,29:60"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
