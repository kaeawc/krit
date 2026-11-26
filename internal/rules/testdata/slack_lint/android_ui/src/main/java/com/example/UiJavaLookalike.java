package com.example;

import android.app.Activity;
import android.content.res.Resources;
import android.view.View;
import java.util.List;

class FakeResources {
  String getQuantityString(int id, int count, Object... args) { return ""; }
}

class Owner {
  Object getContext() { return null; }
}

class UiJavaLookalike {
  View view;
  Resources res;

  void f(List<Owner> owners, List<FakeResources> fakes, FakeResources resources, int n) {
    owners.forEach(view -> { Object x = (Activity) view.getContext(); });
    fakes.forEach(res -> res.getQuantityString(1, n, n));
    for (Owner view : owners) { Object y = (Activity) view.getContext(); }
    resources.getQuantityString(1, n, n);
  }
}
