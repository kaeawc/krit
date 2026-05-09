package fixtures.positive.potentialbugs

interface Greeter {
    fun greet(name: String): String
    fun farewell()
}

class WrongCountGreeter : Greeter {
    // Mismatch: super has 1 param, this has 2.
    override fun greet(name: String, locale: String): String = "$name $locale"

    // Mismatch: super has 0 params, this has 1.
    override fun farewell(message: String) {
    }
}
