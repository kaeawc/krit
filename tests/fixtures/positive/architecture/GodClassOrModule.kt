package architecture

import alpha.analytics.AnalyticsClient
import beta.auth.SessionStore
import gamma.cache.MemoryCache
import delta.config.RuntimeConfig
import epsilon.data.UserRepository
import zeta.di.ServiceLocator
import eta.events.EventBus
import theta.flags.FeatureFlags
import iota.logging.StructuredLogger
import kappa.metrics.MetricsRecorder
import lambda.navigation.Navigator
import mu.notifications.PushRegistrar
import nu.payments.BillingGateway

class AppCoordinator {
    fun start() {
        println("start")
    }
}
