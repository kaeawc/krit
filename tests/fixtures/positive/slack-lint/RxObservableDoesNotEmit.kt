package fixtures.positive.slacklint

import kotlinx.coroutines.rx3.rxObservable

fun names() = rxObservable<String> {
    println("never sends")
}
