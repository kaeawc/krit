package fixtures.negative.potentialbugs

class SmartCastInvalidated {
    // val cannot be reassigned — smart cast holds.
    fun valNotReassigned(s: String?): Int {
        if (s != null) {
            return s.length
        }
        return 0
    }

    // var declared, but never reassigned inside the if-body — smart cast holds.
    fun varNotReassigned(): Int {
        var s: String? = "hi"
        if (s != null) {
            return s.length
        }
        return 0
    }

    // Reassigned but accessed via safe call (?.) — explicitly null-aware.
    fun reassignedSafeCall(): Int {
        var s: String? = "hi"
        if (s != null) {
            s = null
            return s?.length ?: 0
        }
        return 0
    }

    // Use is BEFORE reassignment, not after.
    fun usedBeforeReassignment(): Int {
        var s: String? = "hi"
        if (s != null) {
            val len = s.length
            s = null
            return len
        }
        return 0
    }
}
