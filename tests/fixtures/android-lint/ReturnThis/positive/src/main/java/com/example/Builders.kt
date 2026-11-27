package com.example

import androidx.annotation.ReturnThis

open class Builder {
    @ReturnThis
    open fun name(name: String): Builder {
        if (name.isEmpty()) return Builder()
        return this
    }

    @ReturnThis
    open fun copy(): Builder = Builder()
}

class SubBuilder : Builder() {
    override fun name(name: String): Builder {
        return SubBuilder()
    }
}
