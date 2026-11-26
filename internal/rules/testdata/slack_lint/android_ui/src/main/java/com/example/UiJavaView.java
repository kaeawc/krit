package com.example;

import android.app.Activity;
import android.view.View;
import static android.text.Spanned.SPAN_EXCLUSIVE_EXCLUSIVE;
import static android.text.Spanned.SPAN_POINT_MARK_MASK;

class UiJavaView extends View {
  UiJavaView() { super(null); }
  boolean f(View v, int flags) {
    boolean a = v.getContext() instanceof Activity;
    Activity b = (Activity) getContext();
    boolean c = (flags | SPAN_POINT_MARK_MASK) == SPAN_EXCLUSIVE_EXCLUSIVE;
    return a && c;
  }
  boolean g(int flags, int SPAN_EXCLUSIVE_EXCLUSIVE) {
    return flags == SPAN_EXCLUSIVE_EXCLUSIVE;
  }
}
