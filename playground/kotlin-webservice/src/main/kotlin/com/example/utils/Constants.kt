package com.example.utils

// MagicNumber violations throughout
// StringLiteralDuplication
object Constants {

    val API_VERSION = "v1"
    val BASE_URL = "http://localhost:8080"
    val DEFAULT_PAGE_SIZE = 25
    val MAX_PAGE_SIZE = 100
    val MIN_PASSWORD_LENGTH = 8
    val MAX_PASSWORD_LENGTH = 128
    val SESSION_TIMEOUT_SECONDS = 3600
    val RATE_LIMIT_PER_MINUTE = 60
    val MAX_RETRY_COUNT = 3
    val RETRY_DELAY_MS = 1000
    val CONNECTION_TIMEOUT = 5000
    val READ_TIMEOUT = 10000
    val CACHE_TTL_SECONDS = 300

    // Hardcoded strings that should be configurable
    val DATABASE_HOST = "localhost"
    val DATABASE_PORT = 5432
    val DATABASE_NAME = "myapp"
    val DATABASE_USER = "admin"
    val DATABASE_PASSWORD = "password123"

    val JWT_SECRET = "my-super-secret-key-do-not-share"
    val JWT_EXPIRATION_HOURS = 24

    val SMTP_HOST = "smtp.gmail.com"
    val SMTP_PORT = 587
    val SMTP_USER = "noreply@example.com"

    val ERROR_NOT_FOUND = "Resource not found"
    val ERROR_UNAUTHORIZED = "Unauthorized access"
    val ERROR_BAD_REQUEST = "Bad request"
    val ERROR_INTERNAL = "Internal server error"
    val ERROR_FORBIDDEN = "Forbidden"

    val CONTENT_TYPE_JSON = "application/json"
    val CONTENT_TYPE_XML = "application/xml"

    // MagicNumber: status codes as raw numbers
    fun getStatusMessage(code: Int): String {
        return when (code) {
            200 -> "OK"
            201 -> "Created"
            204 -> "No Content"
            400 -> "Bad Request"
            401 -> "Unauthorized"
            403 -> "Forbidden"
            404 -> "Not Found"
            500 -> "Internal Server Error"
            502 -> "Bad Gateway"
            503 -> "Service Unavailable"
            else -> "Unknown"
        }
    }
}
