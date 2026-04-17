package comments

class Example {
    private val secretField = 42

    /**
     * Uses [secretField] for computation.
     */
    fun compute(): Int {
        return secretField * 2
    }
}
