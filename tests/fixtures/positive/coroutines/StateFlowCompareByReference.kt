package test

import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map

fun observe(state: StateFlow<UiState>) {
    state.map { it.count }.distinctUntilChanged().collect { render(it) }
}

data class UiState(val count: Int)

fun render(count: Int) {}
