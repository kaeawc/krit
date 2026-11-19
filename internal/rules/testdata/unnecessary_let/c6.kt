package test

class Obj(val v: Int) {
    fun f(): Int = v
}

fun use(x: Any?) {}

fun parenthesized(a: Obj?) {
    a?.let { (it) }
    val r = a?.let { (it) }
    use(r)
}

fun boundReference(a: Obj?, b: Obj) {
    a?.let { it::f }
    val r = a?.let { it::f }
    b.let { it::f }
    a?.let { use(it::f) }
    use(r)
}

fun unboundReference(a: Obj?) {
    a?.let { Obj::f }
}
