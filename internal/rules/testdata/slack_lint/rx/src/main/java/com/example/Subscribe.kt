package com.example

import android.os.Looper
import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers
import io.reactivex.rxjava3.core.Completable
import io.reactivex.rxjava3.core.Flowable
import io.reactivex.rxjava3.core.Maybe
import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.core.Scheduler
import io.reactivex.rxjava3.core.Single
import io.reactivex.rxjava3.schedulers.Schedulers

private val topLevelMain = AndroidSchedulers.mainThread()

class Subscribe(private val injected: Scheduler) {
  private val main = AndroidSchedulers.mainThread()
  private val lazyMain by lazy { AndroidSchedulers.mainThread() }

  fun direct(obs: Observable<String>, flow: Flowable<String>, single: Single<Int>, maybe: Maybe<Int>, done: Completable) {
    obs.subscribeOn(AndroidSchedulers.mainThread()).subscribe()
    flow.subscribeOn(AndroidSchedulers.mainThread())
    single.subscribeOn(AndroidSchedulers.mainThread())
    maybe.subscribeOn(AndroidSchedulers.mainThread())
    done.subscribeOn(AndroidSchedulers.mainThread())
    obs.map { it.length }
      .subscribeOn(AndroidSchedulers.mainThread())
  }

  fun notMain(obs: Observable<String>, looper: Looper) {
    obs.subscribeOn(Schedulers.io())
    obs.observeOn(AndroidSchedulers.mainThread())
    obs.subscribeOn(AndroidSchedulers.from(looper))
    obs.subscribeOn(injected)
    obs.subscribeOn(lazyMain)
  }

  fun variables(obs: Observable<String>) {
    obs.subscribeOn(main)
    obs.subscribeOn(topLevelMain)
    val local = AndroidSchedulers.mainThread()
    obs.subscribeOn(local)
  }

  fun parameter(obs: Observable<String>, main: Scheduler) {
    // The parameter shadows the property; Lint does not follow parameters.
    obs.subscribeOn(main)
  }
}
