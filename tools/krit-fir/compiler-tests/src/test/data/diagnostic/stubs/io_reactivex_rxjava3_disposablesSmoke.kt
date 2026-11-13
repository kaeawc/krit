// Smoke: a Disposable is disposed and queried.
package stubs

import io.reactivex.rxjava3.disposables.Disposable

fun disposeOnce(disposable: Disposable): Boolean {
    disposable.dispose()
    return disposable.isDisposed
}
