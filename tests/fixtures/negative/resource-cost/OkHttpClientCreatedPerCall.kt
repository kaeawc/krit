package fixtures.negative.resourcecost

import okhttp3.OkHttpClient

object OkHttpClientCreatedPerCall {
    private val client = OkHttpClient.Builder()
        .connectTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    fun makeRequest(url: String) {
        val request = Request.Builder().url(url).build()
        client.newCall(request).execute()
    }
}
