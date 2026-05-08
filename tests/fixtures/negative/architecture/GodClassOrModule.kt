package architecture

import alpha.analytics.AnalyticsClient
import alpha.analytics.AnalyticsEvent
import beta.auth.SessionStore
import beta.auth.SessionToken
import gamma.cache.MemoryCache
import gamma.cache.MemoryEntry

class FeatureModule {
    fun start() {
        println("start")
    }
}
