// Compiler-test source stubs; never packaged in the production artifact.
package dev.zacsweers.metro

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
annotation class DependencyGraph(
    val scope: KClass<*> = Nothing::class,
    val additionalScopes: Array<KClass<*>> = [],
    val excludes: Array<KClass<*>> = [],
    val bindingContainers: Array<KClass<*>> = [],
) {
    @Target(AnnotationTarget.CLASS)
    annotation class Factory
}

@Target(AnnotationTarget.CLASS)
annotation class GraphExtension(
    val scope: KClass<*> = Nothing::class,
    val additionalScopes: Array<KClass<*>> = [],
    val excludes: Array<KClass<*>> = [],
    val bindingContainers: Array<KClass<*>> = [],
) {
    @Target(AnnotationTarget.CLASS)
    annotation class Factory
}

@Target(AnnotationTarget.CLASS)
annotation class BindingContainer(val includes: Array<KClass<*>> = [])

@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.CONSTRUCTOR,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY,
)
annotation class Inject

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER)
annotation class Provides

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY, AnnotationTarget.PROPERTY_GETTER)
annotation class Binds

@Target(
    AnnotationTarget.CLASS,
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY,
    AnnotationTarget.PROPERTY_GETTER,
)
annotation class SingleIn(val scope: KClass<*>)

@Target(AnnotationTarget.CLASS)
annotation class ContributesTo(val scope: KClass<*>, val replaces: Array<KClass<*>> = [])

@Target(AnnotationTarget.CLASS)
annotation class ContributesBinding(
    val scope: KClass<*>,
    val binding: KClass<*> = Nothing::class,
    val replaces: Array<KClass<*>> = [],
)

abstract class AppScope private constructor()

inline fun <reified T : Any> createGraph(): T = TODO()

inline fun <reified T : Any> createGraphFactory(): T = TODO()
