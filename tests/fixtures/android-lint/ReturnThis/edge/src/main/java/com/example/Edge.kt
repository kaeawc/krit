package com.example

import androidx.annotation.ReturnThis

open class Base {
    @ReturnThis
    open fun parens(): Base = (this)

    @ReturnThis
    open fun other(): Base = Base()
}

@ReturnThis
open class AllSelf {
    open fun chain(): AllSelf = AllSelf()
}
