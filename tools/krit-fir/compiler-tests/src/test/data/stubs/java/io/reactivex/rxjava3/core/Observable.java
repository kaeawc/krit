// Compiler-test source stub; never packaged in the production artifact.
package io.reactivex.rxjava3.core;

import io.reactivex.rxjava3.annotations.CheckReturnValue;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.functions.Consumer;

public abstract class Observable<T> {
    @CheckReturnValue
    public final Disposable subscribe() {
        return null;
    }

    @CheckReturnValue
    public final Disposable subscribe(Consumer<? super T> onNext) {
        return null;
    }

    @CheckReturnValue
    public final Observable<T> doOnSubscribe(Consumer<? super Disposable> onSubscribe) {
        return this;
    }
}
