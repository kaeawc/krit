package test

import kotlinx.coroutines.flow.MutableStateFlow

class VM {
    val state = MutableStateFlow(0)
}
