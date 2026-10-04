// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): a java.security star import resolves the bare
// SecureRandom name to java.security.SecureRandom, so the seeded call reports.
// Go misses it because it needs an explicit java.security.SecureRandom import.
package test

import java.security.*

fun seeded(): SecureRandom = <!TrulyRandom!>SecureRandom(byteArrayOf(1))<!>

fun unseeded(): SecureRandom = SecureRandom()
