package test

fun loadAwsKey(): String? {
    return System.getenv("AWS_ACCESS_KEY_ID")
}
