package com.example

import slack.lint.annotations.JavaOnly
import slack.lint.annotations.KotlinOnly

fun use(c: JavaOnlyClass) {
  JavaApi.javaOnly()
  JavaApi.javaOnlyReason()
  JavaApi.plain()
  c.member()
  val ref = JavaApi::javaOnly
  val anon = object : JavaOnlyCallback {
    override fun call() {}
  }
  val lambda = JavaOnlyCallback { }
}

class Impl : JavaOnlyCallback {
  override fun call() {}
}

class ListenerImpl : Listener {
  override fun onEvent() {}
}

class KImpl : KotlinOnlyBase

@JavaOnly
@KotlinOnly
class Both

class Fns {
  @JavaOnly
  @KotlinOnly
  fun both() {}
}

@JavaOnly
fun provide(): JavaOnlyCallback {
  return object : JavaOnlyCallback {
    override fun call() {}
  }
}
