package style

data class Person(val name: String, val age: Int)

abstract class Shape(val sides: Int)

sealed class Result(val code: Int)
