package style

annotation class Marker

class Holder(@property:Marker val a: Int, @field:Marker val b: Int, @Marker c: Int) {
    constructor(@param:Marker d: String) : this(1, 2, 3)

    @get:Marker val e = 1
    @field:Marker val f = 1
    @param:Marker val g = 1
}

fun use(@param:Marker x: Int) = x
