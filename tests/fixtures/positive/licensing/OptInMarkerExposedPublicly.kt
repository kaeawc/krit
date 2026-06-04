package test

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.SharedFlow

// A public declaration carrying a propagating opt-in marker DIRECTLY exposes
// the opt-in requirement to every caller.
@ExperimentalCoroutinesApi
public fun exposeExperimental(): SharedFlow<Int> = TODO()
