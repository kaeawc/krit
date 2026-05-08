package com.example.exceptions

class MyClass {

    override fun equals(other: Any?): Boolean {
        throw RuntimeException("not implemented")
    }

    override fun hashCode(): Int {
        throw IllegalStateException()
    }

    override fun toString(): String {
        throw RuntimeException("oops")
    }
}
