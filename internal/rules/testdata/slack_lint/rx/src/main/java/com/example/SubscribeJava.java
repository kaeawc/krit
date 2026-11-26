package com.example;

import static io.reactivex.rxjava3.android.schedulers.AndroidSchedulers.mainThread;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.Scheduler;
import io.reactivex.rxjava3.schedulers.Schedulers;

class SubscribeJava {
  private final Scheduler main = AndroidSchedulers.mainThread();
  private final Scheduler unqualified = mainThread();

  void direct(Observable<String> obs) {
    obs.subscribeOn(AndroidSchedulers.mainThread());
    obs.subscribeOn(mainThread());
    obs.subscribeOn(Schedulers.io());
    obs.observeOn(AndroidSchedulers.mainThread());
  }

  void variables(Observable<String> obs, Scheduler param) {
    obs.subscribeOn(main);
    // Lint follows only a qualified initializer, not a static import.
    obs.subscribeOn(unqualified);
    Scheduler local = AndroidSchedulers.mainThread();
    obs.subscribeOn(local);
    obs.subscribeOn(param);
  }
}
