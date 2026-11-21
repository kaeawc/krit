package stubs

import kotlinx.atomicfu.*

class AtomicSmoke {
    var counter by atomic(0)
    var total by atomic(0L)
    var enabled by atomic(false)
    var reference by atomic("before")
    fun update() {
        counter++
        total++
        enabled = true
        reference = "after"
        atomic(0L).value = 1L
        atomic(false).value = true
        atomic("before").value = "after"
    }
}
