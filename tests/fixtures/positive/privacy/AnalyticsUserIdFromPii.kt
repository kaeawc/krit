package test

data class User(
    val email: String,
    val phoneNumber: String,
    val username: String,
    val anonymousId: String,
)

class FirebaseAnalytics {
    fun setUserId(userId: String) {}
}

fun trackUser(firebaseAnalytics: FirebaseAnalytics, user: User) {
    firebaseAnalytics.setUserId(user.email)
    firebaseAnalytics.setUserId(user.phoneNumber)
    firebaseAnalytics.setUserId(user.username)
}
