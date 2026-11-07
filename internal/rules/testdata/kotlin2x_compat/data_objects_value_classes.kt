// rules: ClassNaming, UseDataClass, DataClassShouldBeImmutable
package compat

sealed interface Result {
    data object Loading : Result
    data object lowercase : Result // expect: ClassNaming
    data class Success(val value: Int) : Result
}

data object TopLevel {
    val label = "x"
}

@JvmInline
value class UserId(val raw: Long)

@JvmInline
value class mixedCase(val raw: Int) // expect: ClassNaming

value class Plain(val raw: Int) {
    fun doubled() = raw * 2
}

class NotData(val a: Int, val b: String) // expect: UseDataClass

data class Mutable(var x: Int) // expect: DataClassShouldBeImmutable

data class Immutable(val x: Int)
