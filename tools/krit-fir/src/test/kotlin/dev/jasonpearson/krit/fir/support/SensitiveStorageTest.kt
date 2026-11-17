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
}
