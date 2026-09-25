// Compiler-test source stubs; never packaged in the production artifact.
package dagger.hilt.android

import kotlin.reflect.KClass

// Java @Target(TYPE) with the default CLASS retention.
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class AndroidEntryPoint(val value: KClass<*> = Void::class)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class HiltAndroidApp(val value: KClass<*> = Void::class)
