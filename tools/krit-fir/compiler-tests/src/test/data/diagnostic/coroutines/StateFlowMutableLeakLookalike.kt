// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: a local class named MutableStateFlow is not the kotlinx type.
package test.lookalike

class MutableStateFlow<T>(var value: T)

class ViewModel {
    val state = MutableStateFlow(0)

    val typed: MutableStateFlow<String> = MutableStateFlow("")
}

val topLevel = MutableStateFlow(false)
