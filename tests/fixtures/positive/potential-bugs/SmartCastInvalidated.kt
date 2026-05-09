package fixtures.positive.potentialbugs

class SmartCastInvalidated {
    fun reassignedAndUsed(): Int {
        var x: String? = "hello"
        if (x != null) {
            x = null
            return x.length // smart cast invalidated by reassignment
        }
        return 0
    }

    fun reassignedAndCalled(): Int {
        var s: String? = "world"
        if (s != null) {
            s = compute()
            return s.length // smart cast invalidated by reassignment
        }
        return 0
    }
}

private fun compute(): String? = null
