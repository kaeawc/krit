package test

import java.net.URL

fun safeHttpUrl(endpoint: String) {
    URL("https://api.example.com/")
    URL("http://localhost:8080/")
    URL(endpoint)
}
