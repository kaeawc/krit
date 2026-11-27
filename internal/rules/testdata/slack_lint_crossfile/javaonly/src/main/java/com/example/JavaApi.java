package com.example;

import slack.lint.annotations.JavaOnly;
import slack.lint.annotations.KotlinOnly;

public class JavaApi {
  @JavaOnly public static void javaOnly() {}
  @JavaOnly(reason = "use the Kotlin extension") public static void javaOnlyReason() {}
  public static void plain() {}
}

@JavaOnly
class JavaOnlyClass {
  public void member() {}
}

@JavaOnly
interface JavaOnlyCallback {
  void call();
}

interface Listener {
  @JavaOnly void onEvent();
}

@KotlinOnly
interface KotlinOnlyBase {}
