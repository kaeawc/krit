// Compiler-test source stub; never packaged in the production artifact.
package io.reactivex.rxjava3.processors;

import io.reactivex.rxjava3.annotations.CheckReturnValue;

public final class PublishProcessor<T> {
    @CheckReturnValue
    public boolean offer(T value) {
        return true;
    }

    public void onNext(T value) {
    }
}
