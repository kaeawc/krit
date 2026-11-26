package fixtures.positive.slacklint

import java.util.concurrent.ConcurrentHashMap

class Cache {
    private val entries: ConcurrentHashMap<String, String?> = ConcurrentHashMap()
    private val byKey = ConcurrentHashMap<String?, Int>()
}
