package test

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.distinctUntilChanged

fun observe() {
    MutableStateFlow(0).asStateFlow().distinctUntilChanged().collect { render(it) }
}

fun render(count: Int) {}
