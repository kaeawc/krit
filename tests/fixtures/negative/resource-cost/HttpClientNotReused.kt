package fixtures.negative.resourcecost

import java.net.http.HttpClient

object HttpClientNotReused {
    private val client = HttpClient.newHttpClient()

    fun sendRequest() {
        client.send(null, null)
    }
}
