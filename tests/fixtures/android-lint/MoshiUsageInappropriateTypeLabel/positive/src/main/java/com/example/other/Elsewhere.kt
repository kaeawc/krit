package com.example.other

import com.example.Shape
import dev.zacsweers.moshix.sealed.annotations.TypeLabel

// The moshi-sealed parent is in another package, so Lint does not count it.
@TypeLabel("elsewhere")
data class Elsewhere(val x: Int) : Shape
