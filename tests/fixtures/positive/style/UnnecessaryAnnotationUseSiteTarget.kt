package style

annotation class Marker

class Holder(@param:Marker val a: Int, @param:Marker b: Int) {
    @property:Marker val c = 1
}
