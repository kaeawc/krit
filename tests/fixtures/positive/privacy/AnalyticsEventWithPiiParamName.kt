fun bundleOf(vararg pairs: Any): Any = pairs

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class SignupTracker(
    private val firebaseAnalytics: FirebaseAnalytics,
) {
    fun trackSignup(email: String) {
        firebaseAnalytics.logEvent(
            "signup",
            bundleOf(
                "user_email" to email,
                "plan" to "free",
            ),
        )
    }
}
