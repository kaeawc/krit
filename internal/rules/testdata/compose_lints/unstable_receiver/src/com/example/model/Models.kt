package com.example.model

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.StableMarker

@StableMarker
annotation class CustomStable

@Stable
interface StableBase

class Derived : StableBase

class Plain

@CustomStable
class Marked

@Immutable
data class Frozen(val x: Int)

enum class Kind { A }

@JvmInline
value class Id(val raw: String)

open class Middle : StableBase

class Leaf : Middle()
