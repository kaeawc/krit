package rules_test

import "testing"

func TestBadHostnameVerifierSkipsAbstractVerify(t *testing.T) {
	got := findingPositions(runRuleByName(t, "BadHostnameVerifier", `package p

import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSession

interface NamedVerifier : HostnameVerifier {
  override fun verify(hostname: String, session: SSLSession): Boolean
}

abstract class BaseVerifier : HostnameVerifier {
  abstract override fun verify(hostname: String, session: SSLSession): Boolean
}

class AcceptAll : HostnameVerifier {
  override fun verify(hostname: String, session: SSLSession): Boolean = true
}
`))
	if want := "15:16"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestBadHostnameVerifierJavaSkipsAbstractVerify(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "BadHostnameVerifier", `package p;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLSession;

abstract class BaseVerifier implements HostnameVerifier {
  public abstract boolean verify(String hostname, SSLSession session);
}

class AcceptAll implements HostnameVerifier {
  public boolean verify(String hostname, SSLSession session) { return true; }
}
`))
	if want := "11:18"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsCastMatchesCompleteType(t *testing.T) {
	got := findingPositions(runRuleByName(t, "HardwareIds", `package p

import android.content.Context
import android.telephony.TelephonyManager

class TelephonyManagerFake {
  fun getDeviceId(): String = "fake"
}

fun ids(ctx: Context) {
  val fake = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManagerFake
  fake.getDeviceId()
  val real = ctx.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
  real.getDeviceId()
}
`))
	if want := "14:3"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestHardwareIdsKnownSettingValueWins(t *testing.T) {
	got := findingPositions(runRuleByName(t, "HardwareIds", `package p

import android.content.Context
import android.provider.Settings

const val ANDROID_ID = "other"

fun ids(ctx: Context) {
  Settings.Secure.getString(ctx.contentResolver, ANDROID_ID)
  Settings.Secure.getString(ctx.contentResolver, Settings.Secure.ANDROID_ID)
}
`))
	if want := "10:3"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestUnsafeDynamicallyLoadedCodeJavaLocalSystem(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "UnsafeDynamicallyLoadedCode", `package p;

class Loader {
  static class System {
    static void load(String name) {}
  }

  void run() {
    System.load("lib");
    java.lang.System.load("lib");
  }
}
`))
	if want := "10:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomFilledArraySeedIsNotConstant(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SecureRandom", `package p;

import java.security.SecureRandom;

class A {
  void filled(SecureRandom source) {
    SecureRandom target = new SecureRandom();
    final byte[] seed = new byte[32];
    source.nextBytes(seed);
    target.setSeed(seed);
  }

  void zero() {
    SecureRandom target = new SecureRandom();
    final byte[] seed = new byte[32];
    target.setSeed(seed);
  }
}
`))
	if want := "16:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomKotlinFilledArraySeedIsNotConstant(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SecureRandom", `package p

import java.security.SecureRandom

fun filled(source: SecureRandom) {
  val target = SecureRandom()
  val seed = ByteArray(32)
  source.nextBytes(seed)
  target.setSeed(seed)
}

fun zero() {
  val target = SecureRandom()
  val seed = ByteArray(32)
  target.setSeed(seed)
}
`))
	if want := "15:3"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomParameterShadowsFileConstant(t *testing.T) {
	got := findingPositions(runRuleByName(t, "SecureRandom", `package p

import java.security.SecureRandom

val seed = 7L

fun reseed(seed: Long) {
  val r = SecureRandom()
  r.setSeed(seed)
}

fun fixed() {
  val r = SecureRandom()
  r.setSeed(seed)
}
`))
	if want := "14:3"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestSecureRandomJavaParameterShadowsField(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "SecureRandom", `package p;

import java.security.SecureRandom;

class A {
  static final long SEED = 7L;

  void param(long SEED) {
    SecureRandom r = new SecureRandom();
    r.setSeed(SEED);
  }

  void fixed() {
    SecureRandom r = new SecureRandom();
    r.setSeed(SEED);
  }
}
`))
	if want := "15:5"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
