package test

import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.SharedFlow

@OptIn(ExperimentalCoroutinesApi::class)
public fun exposeExperimental(): SharedFlow<Int> = TODO()
