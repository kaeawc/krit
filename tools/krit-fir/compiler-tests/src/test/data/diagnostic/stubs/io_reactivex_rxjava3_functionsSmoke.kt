// Smoke: a Consumer is a single-method interface that takes a Kotlin lambda.
package stubs

import io.reactivex.rxjava3.functions.Consumer

fun lengthConsumer(sink: MutableList<Int>): Consumer<String> = Consumer { value -> sink.add(value.length) }
