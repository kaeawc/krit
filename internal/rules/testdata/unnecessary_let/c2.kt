package c2

class A { fun foo(): Int = 1 }
class Shadow { fun <T, R> T.let(block: (T) -> R): R = block(this); fun c52b(a: A) { val r = a.let { it.foo() } } }
