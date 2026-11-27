package com.example

import androidx.annotation.RequiresApi
import org.junit.Test

class ApiTest {
    @Test
    fun plain() {}

    @RequiresApi(28)
    fun helper() {}

    @RequiresApi(28)
    private fun privateHelper() {}
}

@RequiresApi(26)
class Helper
