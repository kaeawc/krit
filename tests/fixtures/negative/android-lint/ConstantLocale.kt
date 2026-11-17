package com.example

import java.util.Locale

val currentLocale: Locale
    get() = Locale.getDefault()

fun format(value: Int): String = String.format(Locale.getDefault(), "%d", value)
