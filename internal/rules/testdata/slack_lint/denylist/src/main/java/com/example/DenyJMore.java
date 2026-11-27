package com.example;

import android.view.View;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.Single;
import io.reactivex.rxjava3.schedulers.Schedulers;
import java.text.SimpleDateFormat;
import java.util.*;

class DenyJMore {
  private final Calendar calendar = Calendar.getInstance();

  void f(Observable<String> obs, Single<Integer> single, View view, Date date) {
    obs.hide();
    obs.blockingFirst();
    single.blockingGet();
    Schedulers.newThread();
    new Stack<Integer>();
    new java.util.Vector<Integer>();
    new SimpleDateFormat("yyyy").format(date);
    date.getTime();
    calendar.get(Calendar.YEAR);
    view.setOnClickListener(null);
    new ArrayList<Integer>();
    // new LinkedList<String>();
    String s = "Calendar.YEAR";
  }

  void lookalike(com.other.Date d) {
    d.getTime();
  }
}
