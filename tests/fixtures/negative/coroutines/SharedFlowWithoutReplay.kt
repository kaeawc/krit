package test

import kotlinx.coroutines.flow.MutableSharedFlow

class EventBus {
    private val events = MutableSharedFlow<String>(extraBufferCapacity = 1)
}
