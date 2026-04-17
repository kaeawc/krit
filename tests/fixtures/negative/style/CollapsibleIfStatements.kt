package style

fun example(a: Boolean, b: Boolean) {
    if (a) {
        if (b) {
            doSomething()
        }
    } else {
        doOther()
    }
}

fun doSomething() {}
fun doOther() {}
