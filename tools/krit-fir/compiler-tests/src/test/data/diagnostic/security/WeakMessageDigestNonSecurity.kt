// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 28, 35, 36, 42, 43, 48
// A digest whose output is stored under a checksum-like name, or whose caller is named for an avatar, is not
// a security digest; a password hash or an unnamed digest still reports.
package test

import java.security.MessageDigest

class Registry {
    fun hashToken(raw: String): String {
        val md = MessageDigest.getInstance("MD5")
        val fingerprint = md.digest(raw.toByteArray())
        return fingerprint.joinToString("")
    }

    fun tokenHash(raw: String): String {
        val md = MessageDigest.getInstance("MD5")
        val checksum: ByteArray
        checksum = md.digest(raw.toByteArray())
        return checksum.joinToString("")
    }

    fun avatarUrl(email: String) = MessageDigest.getInstance("MD5").digest(email.toByteArray())

    fun other(b: ByteArray) = <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>.digest(b)

    fun protect(raw: String): String {
        val md = <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        val passwordHash = md.digest(raw.toByteArray())
        return passwordHash.joinToString("")
    }

    // Only a property's initializer names the digest, as in Go: a getter-backed or delegated property
    // named for a cache key or checksum still reports.
    val cacheKey: MessageDigest get() = <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
    val checksumDigest by lazy { <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!> }
    val etagDigest = MessageDigest.getInstance("MD5")

    // A named argument that receives the digest() output names it, as in Go.
    fun named(raw: String) {
        consume(checksum = MessageDigest.getInstance("MD5").digest(raw.toByteArray()))
        consume(secret = <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>.digest(raw.toByteArray()))
        consume(<!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>.digest(raw.toByteArray()))
    }

    // A property does not name a digest() made inside a function in its initializer, as in Go.
    fun local(raw: String): Any {
        val md = <!WeakMessageDigest!>MessageDigest.getInstance("MD5")<!>
        val checksum = object {
            fun bytes() = md.digest(raw.toByteArray())
        }
        return checksum
    }
}

fun consume(checksum: ByteArray? = null, secret: ByteArray? = null) = checksum ?: secret
