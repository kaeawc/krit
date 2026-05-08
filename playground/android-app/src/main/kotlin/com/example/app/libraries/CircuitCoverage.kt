package com.example.app.libraries

import android.os.Parcel
import com.slack.circuit.foundation.Circuit
import com.slack.circuit.runtime.CircuitUiEvent
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.screen.Screen

class CircuitLibraryCoverage {
    val circuit: Circuit =
        Circuit.Builder()
            .build()

    fun screen(): UserListScreen = UserListScreen
}

data object UserListScreen : Screen {
    override fun describeContents(): Int = 0

    override fun writeToParcel(dest: Parcel, flags: Int) {
        dest.writeString("UserListScreen")
    }
}

data class UserListState(
    val users: List<RemoteUser>,
    val eventSink: (UserListEvent) -> Unit,
) : CircuitUiState

sealed interface UserListEvent : CircuitUiEvent {
    data object Refresh : UserListEvent
}
