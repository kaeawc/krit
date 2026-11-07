package rules_test

import (
	"strings"
	"testing"
)

func TestSharedPreferencesForSensitiveKey(t *testing.T) {
	t.Run("flags putString with sensitive key", func(t *testing.T) {
		findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun saveToken(prefs: SharedPreferences, token: String) {
    prefs.edit().putString("auth_token", token).apply()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "auth_token") {
			t.Fatalf("expected mention of auth_token, got %q", findings[0].Message)
		}
	})

	t.Run("allows putString with non-sensitive key", func(t *testing.T) {
		findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun saveTheme(prefs: SharedPreferences, theme: String) {
    prefs.edit().putString("app_theme", theme).apply()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows EncryptedSharedPreferences", func(t *testing.T) {
		findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

object EncryptedSharedPreferences {
    fun create(): SharedPreferences = TODO()
}

fun saveToken(token: String) {
    EncryptedSharedPreferences.create().edit().putString("auth_token", token).apply()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("flags password key", func(t *testing.T) {
		findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun savePassword(prefs: SharedPreferences, pw: String) {
    prefs.edit().putString("password", pw).apply()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
}

func TestPlainFileWriteOfSensitive(t *testing.T) {
	t.Run("flags writeText to credentials file", func(t *testing.T) {
		findings := runRuleByName(t, "PlainFileWriteOfSensitive", `
package test

import java.io.File

fun save(dir: File, json: String) {
    File(dir, "credentials.json").writeText(json)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows writeText to non-sensitive file", func(t *testing.T) {
		findings := runRuleByName(t, "PlainFileWriteOfSensitive", `
package test

import java.io.File

fun save(dir: File, json: String) {
    File(dir, "cache.json").writeText(json)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("flags writeBytes to token file", func(t *testing.T) {
		findings := runRuleByName(t, "PlainFileWriteOfSensitive", `
package test

import java.io.File

fun save(dir: File, data: ByteArray) {
    File(dir, "auth_token.bin").writeBytes(data)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})
}

func TestLogOfSharedPreferenceRead(t *testing.T) {
	t.Run("flags logging SharedPreferences getString with sensitive key", func(t *testing.T) {
		findings := runRuleByName(t, "LogOfSharedPreferenceRead", `
package test

object Log {
    fun d(tag: String, msg: String) {}
}

interface SharedPreferences {
    fun getString(key: String, defValue: String?): String?
}

fun debug(prefs: SharedPreferences) {
    Log.d("Auth", prefs.getString("authToken", null) ?: "")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "authToken") {
			t.Fatalf("expected mention of authToken, got %q", findings[0].Message)
		}
	})

	t.Run("allows logging non-sensitive SharedPreferences key", func(t *testing.T) {
		findings := runRuleByName(t, "LogOfSharedPreferenceRead", `
package test

object Log {
    fun d(tag: String, msg: String) {}
}

interface SharedPreferences {
    fun getString(key: String, defValue: String?): String?
}

fun debug(prefs: SharedPreferences) {
    Log.d("Theme", prefs.getString("theme", "light") ?: "light")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("allows log with plain message", func(t *testing.T) {
		findings := runRuleByName(t, "LogOfSharedPreferenceRead", `
package test

object Log {
    fun d(tag: String, msg: String) {}
}

fun debug() {
    Log.d("Auth", "token loaded")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestSharedPreferencesForSensitiveKey_MatchesWholeWords(t *testing.T) {
	findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun save(prefs: SharedPreferences, v: String) {
    prefs.edit().putString("shipping_address", v).apply()
    prefs.edit().putString("spinner_selection", v).apply()
    prefs.edit().putString("author_name", v).apply()
    prefs.edit().putString("mapping_version", v).apply()
    prefs.edit().putString("refreshToken", v).apply()
    prefs.edit().putString("user_pin", v).apply()
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected refreshToken and user_pin only, got %d: %v", len(findings), findings)
	}
}

func TestSharedPreferencesForSensitiveKeyIgnoresMetadataKeys(t *testing.T) {
	findings := runRuleByName(t, "SharedPreferencesForSensitiveKey", `
package test

interface SharedPreferences {
    interface Editor {
        fun putString(key: String, value: String): Editor
        fun putLong(key: String, value: Long): Editor
        fun putBoolean(key: String, value: Boolean): Editor
        fun putInt(key: String, value: Int): Editor
        fun apply()
    }
    fun edit(): Editor
}

fun save(prefs: SharedPreferences) {
    prefs.edit().putLong("token_expires_at", 1L).apply()
    prefs.edit().putBoolean("has_password", true).apply()
    prefs.edit().putInt("auth_attempt_count", 1).apply()
    prefs.edit().putString("token_type", "x").apply()
    prefs.edit().putString("authToken", "x").apply()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (authToken), got %d: %v", len(findings), findings)
	}
}
