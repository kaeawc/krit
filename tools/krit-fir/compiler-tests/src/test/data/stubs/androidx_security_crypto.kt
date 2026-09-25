// Compiler-test source stubs; never packaged in the production artifact.
package androidx.security.crypto

import android.content.Context
import android.content.SharedPreferences

// Java final class implementing SharedPreferences, created only through its
// static create(...) factories; modeled as an object so `create` keeps its
// Java callable id.
object EncryptedSharedPreferences : SharedPreferences {
    @Deprecated("Use create(Context, String, MasterKey, ...) instead.")
    fun create(
        fileName: String,
        masterKeyAlias: String,
        context: Context,
        prefKeyEncryptionScheme: PrefKeyEncryptionScheme,
        prefValueEncryptionScheme: PrefValueEncryptionScheme,
    ): SharedPreferences = TODO()

    fun create(
        context: Context,
        fileName: String,
        masterKey: MasterKey,
        prefKeyEncryptionScheme: PrefKeyEncryptionScheme,
        prefValueEncryptionScheme: PrefValueEncryptionScheme,
    ): SharedPreferences = TODO()

    override fun getAll(): Map<String, *> = TODO()

    override fun getString(key: String, defValue: String?): String? = TODO()

    override fun getStringSet(key: String, defValues: Set<String>?): Set<String>? = TODO()

    override fun getInt(key: String, defValue: Int): Int = TODO()

    override fun getLong(key: String, defValue: Long): Long = TODO()

    override fun getFloat(key: String, defValue: Float): Float = TODO()

    override fun getBoolean(key: String, defValue: Boolean): Boolean = TODO()

    override fun contains(key: String): Boolean = TODO()

    override fun edit(): SharedPreferences.Editor = TODO()

    override fun registerOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener,
    ) {
        TODO()
    }

    override fun unregisterOnSharedPreferenceChangeListener(
        listener: SharedPreferences.OnSharedPreferenceChangeListener,
    ) {
        TODO()
    }

    enum class PrefKeyEncryptionScheme {
        AES256_SIV,
    }

    enum class PrefValueEncryptionScheme {
        AES256_GCM,
    }
}

class MasterKey private constructor() {
    val isKeyStoreBacked: Boolean
        get() = TODO()

    class Builder(context: Context) {
        constructor(context: Context, keyAlias: String) : this(context)

        fun setKeyScheme(keyScheme: KeyScheme): Builder = TODO()

        fun setUserAuthenticationRequired(authenticationRequired: Boolean): Builder = TODO()

        fun build(): MasterKey = TODO()
    }

    enum class KeyScheme {
        AES256_GCM,
    }

    companion object {
        const val DEFAULT_MASTER_KEY_ALIAS: String = "_androidx_security_master_key_"
    }
}
