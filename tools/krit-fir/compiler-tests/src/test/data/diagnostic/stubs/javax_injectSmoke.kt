// Smoke: javax.inject constructor/field/setter injection, qualifiers, scopes, providers.
package stubs

import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Qualifier
import javax.inject.Scope
import javax.inject.Singleton

@Qualifier
annotation class JavaxQualifier

@Scope
annotation class JavaxScope

@Singleton
class JavaxService @Inject constructor(
    @Named("name") private val name: String,
    @JavaxQualifier private val other: String,
    private val provider: Provider<JavaxClock>,
) {
    @Inject
    lateinit var clock: JavaxClock

    @Inject
    fun setup(@Named("setup") value: String) {
        println(value)
    }

    fun label(): String = name + other + provider.get()
}

@JavaxScope
class JavaxClock @Inject constructor()
