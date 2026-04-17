package style

class MyClass(val name: String)

class AnotherClass(val x: Int, val y: Int)

class PrivateCtor private constructor(val foo: Int)

annotation class Ann

class AnnotatedCtor @Ann constructor(val bar: String)
