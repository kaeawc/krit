// Compiler-test source stubs; never packaged in the production artifact.
package com.google.common.truth

// Truth 1.x: Subject is NOT generic; assertThat is overloaded per actual type
// and returns the matching specialized Subject.
object Truth {
    fun assertThat(actual: Any?): Subject = TODO()

    fun assertThat(actual: Boolean?): BooleanSubject = TODO()

    fun assertThat(actual: String?): StringSubject = TODO()

    fun assertThat(actual: Int?): IntegerSubject = TODO()

    fun assertThat(actual: Long?): LongSubject = TODO()

    fun assertThat(actual: Iterable<*>?): IterableSubject = TODO()

    fun <T : Comparable<*>> assertThat(actual: T?): ComparableSubject<T> = TODO()

    fun assertWithMessage(messageToPrepend: String?): StandardSubjectBuilder = TODO()
}

open class StandardSubjectBuilder {
    fun that(actual: Any?): Subject = TODO()

    fun that(actual: String?): StringSubject = TODO()

    fun that(actual: Boolean?): BooleanSubject = TODO()
}

open class Subject protected constructor() {
    open fun isEqualTo(expected: Any?) {
        TODO()
    }

    open fun isNotEqualTo(unexpected: Any?) {
        TODO()
    }

    fun isNull() {
        TODO()
    }

    fun isNotNull() {
        TODO()
    }

    fun isSameInstanceAs(expected: Any?) {
        TODO()
    }

    fun isInstanceOf(clazz: Class<*>?) {
        TODO()
    }

    fun isIn(iterable: Iterable<*>?) {
        TODO()
    }
}

open class ComparableSubject<T : Comparable<*>> protected constructor() : Subject() {
    fun isGreaterThan(other: T?) {
        TODO()
    }

    fun isLessThan(other: T?) {
        TODO()
    }

    fun isAtLeast(other: T?) {
        TODO()
    }

    fun isAtMost(other: T?) {
        TODO()
    }
}

class IntegerSubject private constructor() : ComparableSubject<Int>()

class LongSubject private constructor() : ComparableSubject<Long>()

class BooleanSubject private constructor() : Subject() {
    fun isTrue() {
        TODO()
    }

    fun isFalse() {
        TODO()
    }
}

class StringSubject private constructor() : ComparableSubject<String>() {
    fun contains(string: CharSequence?) {
        TODO()
    }

    fun startsWith(string: String?) {
        TODO()
    }

    fun endsWith(string: String?) {
        TODO()
    }

    fun isEmpty() {
        TODO()
    }

    fun matches(regex: String?) {
        TODO()
    }
}

open class IterableSubject protected constructor() : Subject() {
    fun isEmpty() {
        TODO()
    }

    fun isNotEmpty() {
        TODO()
    }

    fun hasSize(expectedSize: Int) {
        TODO()
    }

    fun contains(element: Any?) {
        TODO()
    }

    fun containsExactly(vararg varargs: Any?): Ordered = TODO()

    fun containsExactlyElementsIn(expected: Iterable<*>?): Ordered = TODO()
}

interface Ordered {
    fun inOrder()
}
