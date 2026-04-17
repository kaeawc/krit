package test

import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map

fun observe(state: StateFlow<UiState>) {
    state.map { it.count }.collect { render(it) }
}

data class UiState(val count: Int)

fun render(count: Int) {}
