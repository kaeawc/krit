// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR expands the constructor type alias. Go requires an import or FQN that
// directly identifies the Apache class, leaving local aliases unresolved.
package test

typealias UnsafeVerifier = org.apache.http.conn.ssl.AllowAllHostnameVerifier

val verifier = <!AllowAllHostnameVerifier!>UnsafeVerifier<!>()
val constructor = <!AllowAllHostnameVerifier!>::UnsafeVerifier<!>
