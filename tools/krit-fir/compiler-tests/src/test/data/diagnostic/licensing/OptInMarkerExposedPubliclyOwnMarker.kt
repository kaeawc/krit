// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22
package test

import test.ExperimentalCoroutinesApi
import test.ExperimentalCoroutinesApi as Alias

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class ExperimentalLocalApi

@ExperimentalLocalApi
fun apiOwnedByMarkerLibrary() = Unit

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
annotation class ExperimentalCoroutinesApi

// Go-only: Go reports this known name; FIR skips it within the marker's own package tree.
@ExperimentalCoroutinesApi // FIR abstains: Go reports this known name; FIR skips it within the marker's own package tree.
fun apiUsingKnownProjectMarker() = Unit

// Go-only: Go recognizes the qualified name; FIR skips it within the marker's package tree.
@test.ExperimentalCoroutinesApi // FIR abstains: Go recognizes the qualified name; FIR skips it within the marker's package tree.
fun qualifiedSourceMarker() = Unit

@Alias
fun aliasedSourceMarker() = Unit
