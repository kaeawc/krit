// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// Go reports this call because this comment's text says
// import javax.crypto and Go tests the file text. The file imports nothing
// from javax.crypto, java.security, or javax.net.ssl, so FIR reports nothing.
package test

import java.util.Random

fun seeded(): Random = Random(System.nanoTime())
