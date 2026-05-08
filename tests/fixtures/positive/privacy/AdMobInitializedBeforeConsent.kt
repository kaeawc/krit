open class Application {
    open fun onCreate() {}
}

object MobileAds {
    fun initialize(application: Application) {}
}

class ConsentInformation {
    fun requestConsentInfoUpdate() {}
}

class App : Application() {
    private val consentInformation = ConsentInformation()

    override fun onCreate() {
        super.onCreate()
        MobileAds.initialize(this)
        consentInformation.requestConsentInfoUpdate()
    }
}
