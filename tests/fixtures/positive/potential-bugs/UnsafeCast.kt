package potentialbugs

class UnsafeCast {
    fun impossibleAs(): String {
        return 1 as String
    }

    fun impossibleSafeCast(): String? {
        return 1 as? String
    }
}
