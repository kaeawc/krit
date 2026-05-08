package comments

class Example {
    val publicField = 42

    /**
     * Uses [publicField] for computation.
     */
    fun compute(): Int {
        return publicField * 2
    }
}
