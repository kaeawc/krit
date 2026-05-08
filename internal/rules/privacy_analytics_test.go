package rules_test

import (
	"strings"
	"testing"
)

func TestAnalyticsEventWithPiiParamName(t *testing.T) {
	t.Run("flags bundle key matching PII pattern", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsEventWithPiiParamName", `
package test

fun bundleOf(vararg pairs: Any): Any = pairs

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class SignupTracker(private val firebaseAnalytics: FirebaseAnalytics) {
    fun trackSignup(email: String) {
        firebaseAnalytics.logEvent(
            "signup",
            bundleOf(
                "user_email" to email,
                "plan" to "free",
            ),
        )
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "user_email") {
			t.Fatalf("expected mention of user_email, got %q", findings[0].Message)
		}
	})

	t.Run("allows non-PII bundle keys", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsEventWithPiiParamName", `
package test

fun bundleOf(vararg pairs: Any): Any = pairs

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class SignupTracker(private val firebaseAnalytics: FirebaseAnalytics) {
    fun trackSignup() {
        firebaseAnalytics.logEvent(
            "signup",
            bundleOf(
                "plan" to "free",
                "campaign" to "spring-launch",
            ),
        )
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("flags phone number key", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsEventWithPiiParamName", `
package test

fun bundleOf(vararg pairs: Any): Any = pairs

class Analytics {
    fun logEvent(name: String, payload: Any) {}
}

fun track(analytics: Analytics, phone: String) {
    analytics.logEvent("contact", bundleOf("phone" to phone))
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("ignores non-analytics methods", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsEventWithPiiParamName", `
package test

fun bundleOf(vararg pairs: Any): Any = pairs

class Logger {
    fun logMessage(name: String, payload: Any) {}
}

fun track(logger: Logger, email: String) {
    logger.logMessage("contact", bundleOf("email" to email))
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAnalyticsUserIdFromPii(t *testing.T) {
	t.Run("flags setUserId from email property", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsUserIdFromPii", `
package test

data class User(val email: String, val anonymousId: String)

class FirebaseAnalytics {
    fun setUserId(userId: String) {}
}

fun trackUser(firebaseAnalytics: FirebaseAnalytics, user: User) {
    firebaseAnalytics.setUserId(user.email)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "email") {
			t.Fatalf("expected mention of email, got %q", findings[0].Message)
		}
	})

	t.Run("allows setUserId from opaque identifier", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsUserIdFromPii", `
package test

data class User(val anonymousId: String, val installationId: String)

class FirebaseAnalytics {
    fun setUserId(userId: String) {}
}

fun trackUser(firebaseAnalytics: FirebaseAnalytics, user: User) {
    firebaseAnalytics.setUserId(user.anonymousId)
    firebaseAnalytics.setUserId(user.installationId)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("flags phoneNumber and username", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsUserIdFromPii", `
package test

data class User(val phoneNumber: String, val username: String)

class FirebaseAnalytics {
    fun setUserId(userId: String) {}
}

fun trackUser(firebaseAnalytics: FirebaseAnalytics, user: User) {
    firebaseAnalytics.setUserId(user.phoneNumber)
    firebaseAnalytics.setUserId(user.username)
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestCrashlyticsCustomKeyWithPii(t *testing.T) {
	t.Run("flags setCustomKey with PII key name", func(t *testing.T) {
		findings := runRuleByName(t, "CrashlyticsCustomKeyWithPii", `
package test

class FirebaseCrashlytics {
    fun setCustomKey(key: String, value: String) {}
    companion object {
        fun getInstance(): FirebaseCrashlytics = FirebaseCrashlytics()
    }
}

fun logCrashInfo(user: Any) {
    FirebaseCrashlytics.getInstance().setCustomKey("email", "user@example.com")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows setCustomKey with non-PII key", func(t *testing.T) {
		findings := runRuleByName(t, "CrashlyticsCustomKeyWithPii", `
package test

class FirebaseCrashlytics {
    fun setCustomKey(key: String, value: String) {}
    companion object {
        fun getInstance(): FirebaseCrashlytics = FirebaseCrashlytics()
    }
}

fun logCrashInfo() {
    FirebaseCrashlytics.getInstance().setCustomKey("tier", "premium")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestFirebaseRemoteConfigDefaultsWithPii(t *testing.T) {
	t.Run("flags setDefaultsAsync with PII key", func(t *testing.T) {
		findings := runRuleByName(t, "FirebaseRemoteConfigDefaultsWithPii", `
package test

class RemoteConfig {
    fun setDefaultsAsync(defaults: Map<String, Any>) {}
}

fun setup(config: RemoteConfig) {
    config.setDefaultsAsync(mapOf(
        "user_email_template" to "%s@example.com",
    ))
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows setDefaultsAsync without PII key", func(t *testing.T) {
		findings := runRuleByName(t, "FirebaseRemoteConfigDefaultsWithPii", `
package test

class RemoteConfig {
    fun setDefaultsAsync(defaults: Map<String, Any>) {}
}

fun setup(config: RemoteConfig) {
    config.setDefaultsAsync(mapOf(
        "welcome_message" to "Hello",
    ))
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestAnalyticsCallWithoutConsentGate(t *testing.T) {
	t.Run("flags analytics call without consent guard", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsCallWithoutConsentGate", `
package test

object Bundle {
    val EMPTY = Any()
}

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class ScreenTracker(private val firebaseAnalytics: FirebaseAnalytics) {
    fun trackScreenView() {
        firebaseAnalytics.logEvent("screen_view", Bundle.EMPTY)
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows analytics call inside consent guard", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsCallWithoutConsentGate", `
package test

object Bundle {
    val EMPTY = Any()
}

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class ConsentState(val analyticsAllowed: Boolean)

class ScreenTracker(
    private val firebaseAnalytics: FirebaseAnalytics,
    private val consentState: ConsentState,
) {
    fun trackScreenView() {
        if (consentState.analyticsAllowed) {
            firebaseAnalytics.logEvent("screen_view", Bundle.EMPTY)
        }
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows analytics call when function has consent check elsewhere", func(t *testing.T) {
		findings := runRuleByName(t, "AnalyticsCallWithoutConsentGate", `
package test

object Bundle {
    val EMPTY = Any()
}

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class ConsentState(val analyticsAllowed: Boolean)

class Tracker(
    private val analytics: FirebaseAnalytics,
    private val consent: ConsentState,
) {
    fun track() {
        if (!consent.analyticsAllowed) {
            return
        }
        analytics.logEvent("screen_view", Bundle.EMPTY)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}
