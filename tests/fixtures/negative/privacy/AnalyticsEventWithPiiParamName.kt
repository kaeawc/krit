fun bundleOf(vararg pairs: Any): Any = pairs

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class SignupTracker(
    private val firebaseAnalytics: FirebaseAnalytics,
) {
    fun trackSignup() {
        firebaseAnalytics.logEvent(
            "signup",
            bundleOf(
                "plan" to "free",
                "campaign" to "spring-launch",
            ),
        )
    }
}
