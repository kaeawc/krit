package com.example;

import androidx.annotation.Nullable;
import java.util.concurrent.ConcurrentHashMap;

class JavaMaps {
  ConcurrentHashMap<@Nullable String, Integer> field;
  void f() {
    ConcurrentHashMap<String, @Nullable Integer> local = new ConcurrentHashMap<@Nullable String, Integer>();
  }
}
