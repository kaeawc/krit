// Smoke: offer reports whether the processor accepted the item.
package stubs

import io.reactivex.rxjava3.processors.PublishProcessor

fun publish(processor: PublishProcessor<String>): Boolean {
    processor.onNext("a")
    return processor.offer("b")
}
