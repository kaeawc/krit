package test

import java.net.URL
import okhttp3.Request
import retrofit2.Retrofit

fun hardcodedHttpUrls() {
    Retrofit.Builder().baseUrl("http://api.example.com/").build()
    Request.Builder().url("http://cdn.example.com/file").build()
    URL("http://files.example.com/data")
}
