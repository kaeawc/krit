package com.example

import slack.lint.annotations.JavaOnly
import slack.lint.annotations.KotlinOnly

/** A Kotlin class annotated @JavaOnly: its members are Java-only too. */
@JavaOnly
class KotlinJavaOnly {
  fun member() = 1
}

/** Documented subclass. */
open class Middle : KotlinOnlyBase

class Leaf : Middle()

class Holder {
  @JavaOnly(reason = "call the suspend variant")
  fun legacy() = 1

  fun legacy(x: Int) = x
}

interface SamListener {
  @JavaOnly fun onEvent()
}

fun more(k: KotlinJavaOnly, h: Holder) {
  k.member()
  h.legacy()
  h.legacy(1)
  SamListener { }
  val notAnnotated = Runnable { }
}

class Lookalike {
  fun javaOnly() = 1
}

fun lookalike(l: Lookalike) = l.javaOnly()
