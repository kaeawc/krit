package com.example

// This file never mentions Moshi, but Bird extends a moshi-sealed type.
data class Bird(val wings: Int) : Animal()
