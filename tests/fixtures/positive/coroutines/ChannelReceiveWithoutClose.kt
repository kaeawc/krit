package coroutines

import kotlinx.coroutines.channels.Channel

data class Event(val value: String)

class Worker {
    private val events = Channel<Event>()

    fun send(event: Event) {
        events.trySend(event)
    }
}
