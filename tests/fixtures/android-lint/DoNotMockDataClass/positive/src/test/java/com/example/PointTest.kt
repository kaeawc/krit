package com.example

import org.mockito.Mock
import org.mockito.kotlin.mock

class PointTest {
    @Mock lateinit var field: Point

    private val local: Point = mock()
}
