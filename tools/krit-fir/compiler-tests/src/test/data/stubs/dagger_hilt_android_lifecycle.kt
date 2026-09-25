// Compiler-test source stubs; never packaged in the production artifact.
package dagger.hilt.android.lifecycle

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class HiltViewModel(val assistedFactory: KClass<*> = Any::class)
