// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 28
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
}
