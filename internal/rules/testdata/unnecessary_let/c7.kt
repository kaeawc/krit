package test

class Obj(val v: Int)

fun use(x: Any?) {}

fun lambdaValue(xs: List<Obj>, a: Obj?) {
    val first = xs.firstNotNullOfOrNull { x ->
        use(x)
        a?.let { x.v }
    }
    val withDefault = xs.firstNotNullOfOrNull { x ->
        use(x)
        a?.let { x.v }
    } ?: 0
    use(first)
    use(withDefault)
}

fun lambdaUnit(xs: List<Obj>, a: Obj?) {
    xs.forEach { x ->
        use(x)
        a?.let { use(x.v) }
    }
    val same = xs.onEach { x ->
        use(x)
        a?.let { use(x.v) }
    }
    use(same)
}
