package fixtures.positive.slacklint

import java.util.Date
import java.util.LinkedList

fun stamp(): Long {
    val queue = LinkedList<Int>()
    queue.add(1)
    return Date().getTime()
}
