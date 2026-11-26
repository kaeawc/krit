package com.example;

import android.content.Context;
import androidx.startup.Initializer;
import java.util.Collections;
import java.util.List;

class JavaListed implements Initializer<String> {
    @Override public String create(Context context) { return ""; }
    @Override public List<Class<? extends Initializer<?>>> dependencies() { return Collections.emptyList(); }
}

class JavaArgs implements Initializer<String> {
    JavaArgs(String s) {}
    @Override public String create(Context context) { return ""; }
    @Override public List<Class<? extends Initializer<?>>> dependencies() { return Collections.emptyList(); }
}

class JavaDefault implements Initializer<String> {
    @Override public String create(Context context) { return ""; }
    @Override public List<Class<? extends Initializer<?>>> dependencies() { return Collections.emptyList(); }
}
