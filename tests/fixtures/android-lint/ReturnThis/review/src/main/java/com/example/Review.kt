package com.example

import androidx.annotation.ReturnThis

open class Chain {
    // A non-local return from an inline lambda leaves the method.
    @ReturnThis
    open fun viaRun(): Chain {
        run { return Chain() }
        return this
    }

    @ReturnThis
    open fun copy(x: Int): Chain = this

    open fun copy(s: String): Chain = this
}

class Overloads : Chain() {
    // Overrides the annotated copy(Int).
    override fun copy(x: Int): Chain = Chain()

    // Overrides copy(String), which is not annotated.
    override fun copy(s: String): Chain = Chain()
}

interface Top {
    @ReturnThis
    fun top(): Top
}

interface Left : Top

interface Right

// The annotated method comes through the first of two supertypes.
class Both : Left, Right {
    override fun top(): Top = Both()
}
