package test

class Sample {
    fun normalize(userName: String, email: String): String {
        val upper = userName.uppercase()
        val lower = email.lowercase()
        return upper + lower
    }
}
