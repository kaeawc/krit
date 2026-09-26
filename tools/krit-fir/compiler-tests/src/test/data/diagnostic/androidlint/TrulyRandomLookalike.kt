// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Negative: a same-package class named SecureRandom, with no java.security
// import, is not java.security.SecureRandom. Go leaves it alone too (it needs
// the import or the qualified name). The qualified JDK call still reports.
package test

class SecureRandom(val seed: ByteArray)

fun lookalike(): SecureRandom = SecureRandom(byteArrayOf(1, 2, 3))

fun jdk(): java.security.SecureRandom = <!TrulyRandom!>java.security.SecureRandom(byteArrayOf(1))<!>
