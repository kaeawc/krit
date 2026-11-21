package test

object Counter {
    var total = 0
    var endpoint = "x"
        private set
    fun update() { endpoint = "y" }
}
