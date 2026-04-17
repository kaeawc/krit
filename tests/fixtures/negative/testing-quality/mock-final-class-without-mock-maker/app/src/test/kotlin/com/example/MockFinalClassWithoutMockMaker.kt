package com.example

import org.mockito.Mockito.mock

class FinalService {
    fun fetch(): String = "value"
}

fun buildMock(): FinalService {
    return mock(FinalService::class.java)
}
