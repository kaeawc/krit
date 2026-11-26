package com.example;

import java.text.DateFormat;
import java.util.Date;
import java.util.LinkedList;
import static java.util.Calendar.getInstance;
import io.reactivex.rxjava3.core.Completable;

class DenyJ {
  void f(Completable c) {
    new LinkedList<String>();
    new Date();
    getInstance();
    int s = DateFormat.SHORT;
    c.blockingAwait();
  }
}
