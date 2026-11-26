package fixtures.negative.slacklint

import java.util.concurrent.ConcurrentHashMap

class Cache {
    private val entries: ConcurrentHashMap<String, String> = ConcurrentHashMap()
    private val other: HashMap<String?, Int?> = HashMap()
}
