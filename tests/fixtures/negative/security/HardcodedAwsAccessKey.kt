package test

fun loadAwsKey(): String? {
    return System.getenv("AWS_ACCESS_KEY_ID")
}

// AWS documentation example marker — must not be flagged.
const val DOCS_EXAMPLE = "AKIAIOSFODNN7EXAMPLE"
