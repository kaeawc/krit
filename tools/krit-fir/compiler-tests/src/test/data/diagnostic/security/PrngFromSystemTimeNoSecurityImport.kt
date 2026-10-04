// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: the file imports nothing from javax.crypto, java.security, or
// javax.net.ssl, so neither Go nor FIR treats it as security-sensitive code,
// even when it names a security class by its qualified name.
package test

import java.util.Random

fun seeded(): Random = Random(System.nanoTime())

fun digest(): java.security.MessageDigest = java.security.MessageDigest.getInstance("SHA-256")
