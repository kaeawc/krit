// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
// Positive: a SAM-constructor lambda that always returns true is reported on
// the call, as Go reports it.
package test

import javax.net.ssl.HostnameVerifier

val trustAll = <!AllowAllHostnameVerifier!>HostnameVerifier { _, _ -> true }<!>
