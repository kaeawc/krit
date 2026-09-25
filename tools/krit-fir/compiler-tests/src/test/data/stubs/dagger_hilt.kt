// Compiler-test source stubs; never packaged in the production artifact.
package dagger.hilt

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class InstallIn(vararg val value: KClass<*>)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class EntryPoint

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class DefineComponent(val parent: KClass<*> = Void::class)

object EntryPoints {
    fun <T> get(component: Any, entryPoint: Class<T>): T = TODO()
}
