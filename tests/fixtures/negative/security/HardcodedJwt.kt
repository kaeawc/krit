package test

fun authHeader(token: String): String = token

fun placeholder(): String =
    "eyJhbGciOiJIUzI1NiJ9.YOUR_TOKEN_HERE.signaturebody"
