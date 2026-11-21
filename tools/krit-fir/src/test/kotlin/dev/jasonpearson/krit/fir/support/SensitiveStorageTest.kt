package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertEquals

class SensitiveStorageTest {
    // Expected values come from Go's sensitiveStorageKey on the same keys.
    private val goVerdicts = mapOf(
        "auth_token" to true,
        "pin" to true,
        "api_keys" to true,
        "apiKey" to true,
        "userPassword" to true,
        "client_secret" to true,
        "OAUTH" to true,
        "token_\$userId" to true,
        "authToken" to true,
        "API_KEY" to true,
        "privateKey" to true,
        "PINCode" to true,
        "myPIN" to true,
        "XMLToken" to true,
        "credentials" to true,
        "passwd" to true,
        "auth" to true,
        "access-token" to true,
        "access.token" to true,
        "pin.code" to true,
        "app_theme" to false,
        "shipping_address" to false,
        "spinner_position" to false,
        "tokenizer_mode" to false,
        "token_expires_at" to false,
        "password_attempts" to false,
        "has_pin" to false,
        "tokenCount" to false,
        "isAuthEnabled" to false,
        "token2" to false,
        "TOKEN2" to false,
        "" to false,
    )

    @Test
    fun sensitiveKeysFollowGo() {
        for ((key, expected) in goVerdicts) {
            assertEquals(expected, isSensitiveStorageKey(key), "key \"$key\"")
        }
    }

    @Test
    fun sensitiveFileNamesFollowGo() {
        val verdicts = mapOf(
            "credentials.json" to true,
            "auth_token.bin" to true,
            "userPassword.txt" to true,
            "private-key.pem" to true,
            "OAUTH.json" to true,
            "token_expires_at.txt" to true,
            "password_attempts.txt" to true,
            "cache.json" to false,
            "pin.txt" to false,
            "api_key.txt" to false,
            "tokenizer.txt" to false,
            "shipping.txt" to false,
            "token2.txt" to false,
            "" to false,
        )
        for ((path, expected) in verdicts) {
            assertEquals(expected, isSensitiveFileName(path), "path \"$path\"")
        }
    }
}
