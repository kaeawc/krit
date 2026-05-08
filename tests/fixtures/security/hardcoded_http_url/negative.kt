package test

import okhttp3.Request
import retrofit2.Retrofit

class URL(value: String)
class LocalBuilder {
    fun url(value: String) = this
}

fun safeUrls(endpoint: String) {
    Retrofit.Builder().baseUrl("https://api.example.com/").build()
    Retrofit.Builder().baseUrl("http://localhost:8080/").build()
    Retrofit.Builder().baseUrl("http://10.0.2.2:8080/").build()
    Request.Builder().url(endpoint).build()
    URL("http://files.example.com/data")
    LocalBuilder().url("http://api.example.com/")
}
