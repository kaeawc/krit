package coroutines

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class ImageCache {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}
