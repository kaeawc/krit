package io.reactivex.rxjava3.android.schedulers;

import io.reactivex.rxjava3.core.Scheduler;

public final class AndroidSchedulers {
  public static Scheduler mainThread() { return null; }
  public static Scheduler from(android.os.Looper looper) { return null; }
}
