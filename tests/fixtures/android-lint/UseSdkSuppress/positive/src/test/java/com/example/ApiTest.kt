package com.example

import androidx.annotation.RequiresApi
import org.junit.Test

@RequiresApi(26)
class ApiTest {
    @Test
    @RequiresApi(api = 28)
    fun usesNewApi() {}
}
