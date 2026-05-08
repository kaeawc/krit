package fixtures.positive.resourcecost

import java.net.http.HttpClient

class HttpClientNotReused {
    fun sendRequest() {
        val client = HttpClient.newHttpClient()
        client.send(null, null)
    }
}
