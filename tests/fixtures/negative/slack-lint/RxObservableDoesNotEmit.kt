package fixtures.negative.slacklint

import kotlinx.coroutines.rx3.rxObservable

fun names() = rxObservable<String> {
    send("first")
    println("sent")
}
