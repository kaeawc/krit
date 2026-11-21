package io.reactivex.rxjava3.functions;

@FunctionalInterface
public interface Function<T, R> {
    R apply(T value) throws Throwable;
}
