package com.example

import androidx.annotation.VisibleForTesting

class UserRepository {
    fun get(id: Long): String = id.toString()
}

internal class InternalOnly

@VisibleForTesting
class TestHook
