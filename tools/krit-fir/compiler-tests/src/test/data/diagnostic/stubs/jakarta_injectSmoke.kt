// Smoke: jakarta.inject constructor/field injection, qualifiers, scopes, providers.
package stubs

import jakarta.inject.Inject
import jakarta.inject.Named
import jakarta.inject.Provider
import jakarta.inject.Qualifier
import jakarta.inject.Scope
import jakarta.inject.Singleton

@Qualifier
annotation class JakartaQualifier

@Scope
annotation class JakartaScope

@Singleton
class JakartaService @Inject constructor(
    @Named("name") private val name: String,
    @JakartaQualifier private val other: String,
    private val provider: Provider<JakartaClock>,
) {
    @Inject
    lateinit var clock: JakartaClock

    fun label(): String = name + other + provider.get()
}

@JakartaScope
class JakartaClock @Inject constructor()
