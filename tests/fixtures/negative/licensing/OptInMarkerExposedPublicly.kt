package test

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.SharedFlow

// @OptIn(...) CONSUMES the opt-in requirement locally and does NOT propagate it
// to callers; it is the mechanism for NOT exposing the marker. Flagging it is a
// false positive, even on a public declaration.
@OptIn(ExperimentalCoroutinesApi::class)
public fun consumesExperimental(): SharedFlow<Int> = TODO()

// A propagating marker on a non-public declaration does not expose anything
// to external callers.
@ExperimentalCoroutinesApi
private fun internalUse() = Unit
