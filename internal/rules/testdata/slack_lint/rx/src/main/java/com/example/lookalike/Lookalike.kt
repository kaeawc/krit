package com.example.lookalike

import io.reactivex.rxjava3.core.Observable

// A local factory that only shares the coroutines-rx name.
fun <T : Any> rxObservable(block: () -> Unit): Observable<T> = TODO()

class Lookalike {
  fun use() {
    rxObservable<String> { }
  }
}
