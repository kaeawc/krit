package fixtures.positive.slacklint

import kotlinx.coroutines.rx3.rxFlowable

fun names() = rxFlowable<String> {
    println("never sends")
}
