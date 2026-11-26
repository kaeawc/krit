package fixtures.negative.slacklint

import kotlinx.coroutines.rx3.rxFlowable

fun names() = rxFlowable<String> {
    trySend("first")
}
