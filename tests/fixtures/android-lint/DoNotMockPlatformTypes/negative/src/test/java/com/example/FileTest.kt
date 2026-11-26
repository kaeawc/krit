package com.example

import java.io.File
import org.mockito.Mockito

class FileTest {
    private val file = File("data.txt")
    private val repository = Mockito.mock(Repository::class.java)
}
