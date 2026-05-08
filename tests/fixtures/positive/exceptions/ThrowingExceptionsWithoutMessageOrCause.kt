package com.example.exceptions

class Validator {

    fun validate(input: String) {
        if (input.isEmpty()) {
            throw IllegalArgumentException()
        }
        if (input.length > 100) {
            throw IllegalStateException()
        }
    }
}
