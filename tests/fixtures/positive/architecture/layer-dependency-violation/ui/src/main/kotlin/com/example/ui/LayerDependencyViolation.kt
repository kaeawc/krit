package com.example.ui

import com.example.data.internal.InternalSessionStore
import com.example.domain.UserSession

class LayerDependencyViolation(
    private val store: InternalSessionStore,
) {
    fun currentUser(): UserSession? = store.currentUser()
}
