// rules: MutableStateInObject
package compat

import kotlin.concurrent.atomics.AtomicInt
import kotlin.concurrent.atomics.AtomicReference

object Registry {
    var counter = AtomicInt(0)
    var current = AtomicReference<String?>(null)
    var plain = 0 // expect: MutableStateInObject
}
