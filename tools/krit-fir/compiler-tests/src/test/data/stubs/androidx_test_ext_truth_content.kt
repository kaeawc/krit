package androidx.test.ext.truth.content

class IntentSubject {
    fun hasAction(action: String) = Unit
    companion object {
        fun assertThat(intent: Any): IntentSubject = TODO()
    }
}
