package test

data class User(
    val anonymousId: String,
    val installationId: String,
)

class FirebaseAnalytics {
    fun setUserId(userId: String) {}
}

fun trackUser(firebaseAnalytics: FirebaseAnalytics, user: User) {
    firebaseAnalytics.setUserId(user.anonymousId)
    firebaseAnalytics.setUserId(user.installationId)
}
