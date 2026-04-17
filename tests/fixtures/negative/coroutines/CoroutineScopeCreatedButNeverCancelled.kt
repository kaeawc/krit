package coroutines

import java.io.Closeable
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel

class ImageCache : Closeable {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun close() {
        scope.cancel()
    }
}
