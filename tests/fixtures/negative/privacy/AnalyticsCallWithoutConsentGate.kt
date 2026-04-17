object Bundle {
    val EMPTY = Any()
}

class FirebaseAnalytics {
    fun logEvent(name: String, payload: Any) {}
}

class ConsentState(val analyticsAllowed: Boolean)

class ScreenTracker(
    private val firebaseAnalytics: FirebaseAnalytics,
    private val consentState: ConsentState,
) {
    fun trackScreenView() {
        if (consentState.analyticsAllowed) {
            firebaseAnalytics.logEvent("screen_view", Bundle.EMPTY)
        }
    }
}
