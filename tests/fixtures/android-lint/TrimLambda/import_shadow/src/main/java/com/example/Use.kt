package com.example

import com.example.util.trim

fun clean(s: String) = s.trim { it <= ' ' }
