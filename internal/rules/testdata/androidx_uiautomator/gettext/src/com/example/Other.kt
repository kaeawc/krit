package com.example.other

class Other { val text = "x"; fun onView(block: Other.() -> Boolean) = block() }

fun other(o: Other) {
    o.onView { text == "x" }
}
