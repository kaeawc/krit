package fixtures.negative.slacklint

import java.time.Instant
import java.util.ArrayDeque

fun stamp(): Long {
    val queue = ArrayDeque<Int>()
    queue.add(1)
    // Date().getTime() would be reported
    return Instant.now().toEpochMilli()
}
