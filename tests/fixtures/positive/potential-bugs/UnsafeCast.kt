package potentialbugs

class UnsafeCast {
    fun impossibleAs(): String {
        return 1 as String
    }

    fun impossibleSafeCast(): String? {
        return 1 as? String
    }

    // Two unrelated final classes: neither is a subtype of the other.
    fun unrelatedFinalClasses(apple: UnsafeCastApple): UnsafeCastBrick {
        return apple as UnsafeCastBrick
    }
}

class UnsafeCastApple(val n: Int)
class UnsafeCastBrick(val n: Int)
