package coroutines

import java.io.Closeable
import kotlinx.coroutines.channels.Channel

data class Event(val value: String)

class Worker : Closeable {
    private val events = Channel<Event>()

    fun send(event: Event) {
        events.trySend(event)
    }

    override fun close() {
        events.close()
    }
}
