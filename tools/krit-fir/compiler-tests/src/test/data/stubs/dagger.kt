// Compiler-test source stubs; never packaged in the production artifact.
package dagger

import javax.inject.Scope
import kotlin.reflect.KClass

// Dagger's annotations are Java @Retention(RUNTIME); Java METHOD maps to
// FUNCTION plus the property accessors, TYPE to CLASS, PARAMETER to
// VALUE_PARAMETER.

@MustBeDocumented
@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class Provides

@MustBeDocumented
@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class Binds

@MustBeDocumented
@Target(AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.PROPERTY_SETTER)
@Retention(AnnotationRetention.RUNTIME)
annotation class BindsOptionalOf

@MustBeDocumented
@Target(
    AnnotationTarget.FUNCTION,
    AnnotationTarget.PROPERTY_GETTER,
    AnnotationTarget.PROPERTY_SETTER,
    AnnotationTarget.VALUE_PARAMETER,
)
@Retention(AnnotationRetention.RUNTIME)
annotation class BindsInstance

@MustBeDocumented
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Module(
    val includes: Array<KClass<*>> = [],
    val subcomponents: Array<KClass<*>> = [],
)

@MustBeDocumented
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Component(
    val modules: Array<KClass<*>> = [],
    val dependencies: Array<KClass<*>> = [],
) {
    @MustBeDocumented
    @Target(AnnotationTarget.CLASS)
    @Retention(AnnotationRetention.RUNTIME)
    annotation class Builder

    @MustBeDocumented
    @Target(AnnotationTarget.CLASS)
    @Retention(AnnotationRetention.RUNTIME)
    annotation class Factory
}

// An annotation (the previous stub declared a class, which cannot annotate).
@MustBeDocumented
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class Subcomponent(val modules: Array<KClass<*>> = []) {
    @MustBeDocumented
    @Target(AnnotationTarget.CLASS)
    @Retention(AnnotationRetention.RUNTIME)
    annotation class Builder

    @MustBeDocumented
    @Target(AnnotationTarget.CLASS)
    @Retention(AnnotationRetention.RUNTIME)
    annotation class Factory
}

@MustBeDocumented
@Target(AnnotationTarget.ANNOTATION_CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class MapKey(val unwrapValue: Boolean = true)

@MustBeDocumented
@Retention(AnnotationRetention.RUNTIME)
@Scope
annotation class Reusable

// Java `interface Lazy<T> { T get(); }`: invariant in T.
interface Lazy<T> {
    fun get(): T
}

interface MembersInjector<T> {
    fun injectMembers(instance: T)
}
