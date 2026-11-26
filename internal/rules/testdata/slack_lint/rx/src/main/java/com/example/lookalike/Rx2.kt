package com.example.lookalike

import io.reactivex.Observable
import io.reactivex.android.schedulers.AndroidSchedulers

// RxJava 2 types are not in Lint's owner list.
class Rx2 {
  fun use(obs: Observable<String>) {
    obs.subscribeOn(AndroidSchedulers.mainThread())
  }
}
