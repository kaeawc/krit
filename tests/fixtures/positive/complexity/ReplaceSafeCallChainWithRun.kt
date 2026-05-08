package complexity

data class A(val b: B?)
data class B(val c: C?)
data class C(val value: String?)

fun example(a: A?) {
    val result = a?.b?.c?.value
}
