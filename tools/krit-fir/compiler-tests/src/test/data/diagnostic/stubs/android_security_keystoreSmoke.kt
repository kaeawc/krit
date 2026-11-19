// Smoke: KeyProperties algorithm, block mode, padding and purpose constants.
package stubs

import android.security.keystore.KeyProperties

fun keystoreConstants(): String {
    val purposes: Int = KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT or
        KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY
    val algorithms: String = KeyProperties.KEY_ALGORITHM_RSA + KeyProperties.KEY_ALGORITHM_EC +
        KeyProperties.KEY_ALGORITHM_AES + KeyProperties.KEY_ALGORITHM_HMAC_SHA256
    return "$purposes$algorithms${KeyProperties.BLOCK_MODE_ECB}${KeyProperties.BLOCK_MODE_GCM}" +
        KeyProperties.ENCRYPTION_PADDING_NONE
}
