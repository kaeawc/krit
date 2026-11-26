package com.example;

import android.content.Context;
import androidx.startup.Initializer;
import java.util.Collections;
import java.util.List;

class JComment implements Initializer<String> {
    JComment(/* none */) {}
    JComment(String s) {}
    @Override public String create(Context context) { return ""; }
    @Override public List<Class<? extends Initializer<?>>> dependencies() { return Collections.emptyList(); }
}
