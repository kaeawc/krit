package com.example.ui

import com.example.domain.UserSessionReader

class LayerDependencyViolation(
    private val reader: UserSessionReader,
) {
    fun currentUserId(): String? = reader.currentUserId()
}
