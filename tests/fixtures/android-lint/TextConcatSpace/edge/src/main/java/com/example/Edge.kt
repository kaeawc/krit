package com.example

val name = "x"
val a = "ends with escape\n" +
    "next line"
val b = "two words\nthen more words" +
    "next"
val c = "two words" +
    "\tnext"
val d = "prefix $name and words" +
    "${name}next"
val e = """raw two words""" +
    """raw next"""
val f = "several words here" +
    // a comment in between
    "next"
fun g(s: String) = s + "trailing words" +
    "more"
val h = "élan vital déjà" +
    "vu"
