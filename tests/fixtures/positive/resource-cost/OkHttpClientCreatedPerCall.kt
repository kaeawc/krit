package fixtures.positive.resourcecost

import okhttp3.OkHttpClient

class OkHttpClientCreatedPerCall {
    fun makeRequest(url: String) {
        val client = OkHttpClient.Builder()
            .connectTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
            .build()
        val request = Request.Builder().url(url).build()
        client.newCall(request).execute()
    }
}
