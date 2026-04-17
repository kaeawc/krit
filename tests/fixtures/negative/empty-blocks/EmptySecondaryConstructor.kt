package fixtures.negative.emptyblocks

class Foo(val name: String) {
    constructor(x: Int) : this(x.toString()) {
        init(x)
    }
}

class Bar(val x: Int, val y: Int) {
    // Delegation constructor without body — NOT empty
    constructor(x: Int) : this(x, 0)
}

class Baz(val x: Int) {
    // Delegation to super without body — NOT empty
    constructor() : super()
}
