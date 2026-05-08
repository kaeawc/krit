package fixtures.positive.emptyblocks

class Foo(val name: String) {
    constructor(x: Int) : this(x.toString()) { }
}
