package com.example

import com.squareup.moshi.JsonClass
import dev.zacsweers.moshix.sealed.annotations.DefaultObject
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

@TypeLabel("orphan")
data class Orphan(val x: Int)

open class Base

@DefaultObject
object FromOpen : Base()

sealed class PlainSealed

@TypeLabel("plain")
class FromPlainSealed : PlainSealed()
