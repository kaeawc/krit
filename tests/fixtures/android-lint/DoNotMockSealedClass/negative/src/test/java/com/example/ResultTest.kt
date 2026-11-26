package com.example

import org.mockito.kotlin.mock

class ResultTest {
    // Mock a subtype, or better, build one.
    private val result: Result = Success(1)
    private val repository = mock<Repository>()
}
