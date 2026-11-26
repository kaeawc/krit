package com.example;

import android.app.Activity;
import android.content.res.Resources;
import android.text.Spanned;
import android.view.View;

class UiJava {
  void f(View view, Resources res, int flags, int count) {
    Activity a = (Activity) view.getContext();
    res.getQuantityString(1, count, count);
    String formatted = UiKt.getFormattedCount(count);
    res.getQuantityString(1, count, formatted);
    if (flags == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE) {}
    if ((flags & Spanned.SPAN_POINT_MARK_MASK) == Spanned.SPAN_EXCLUSIVE_EXCLUSIVE) {}
  }
}
