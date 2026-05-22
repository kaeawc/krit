package dev.jasonpearson.krit.intellij

sealed class KritState {
    object Initializing : KritState()
    object Scanning : KritState()
    data class Idle(val findingCount: Int) : KritState()
    object MissingBinary : KritState()
    data class Error(val message: String) : KritState()
}

object KritStatusText {
    fun render(state: KritState): String = when (state) {
        is KritState.Initializing -> "Krit: starting…"
        is KritState.Scanning -> "Krit: scanning…"
        is KritState.Idle -> "Krit: ${state.findingCount}"
        is KritState.MissingBinary -> "Krit: binary not found"
        is KritState.Error -> "Krit: error"
    }

    fun tooltip(state: KritState): String = when (state) {
        is KritState.Initializing -> "Krit is starting up; click to configure"
        is KritState.Scanning -> "Krit is analyzing the project; click to configure"
        is KritState.Idle ->
            if (state.findingCount == 0) "Krit: no findings; click to configure"
            else "Krit: ${state.findingCount} finding${if (state.findingCount == 1) "" else "s"}; click to configure"
        is KritState.MissingBinary ->
            "Krit binary not found on PATH. Set KRIT_BINARY or -Dkrit.binary; click to configure."
        is KritState.Error -> "Krit error: ${state.message}; click for the IDE log"
    }
}
