package com.example.util

fun String.trim(predicate: (Char) -> Boolean): String = filterNot(predicate)
