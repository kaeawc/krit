// Smoke: Observable subscriptions return a Disposable and operators return the Observable.
package stubs

import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.disposables.Disposable

fun subscribeAll(source: Observable<String>): Disposable {
    val handlers: Disposable = source.subscribe { value -> value.length }
    val plain: Disposable = source.doOnSubscribe { it.dispose() }.subscribe()
    return if (handlers.isDisposed) plain else handlers
}
