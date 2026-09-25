// Compiler-test source stubs; never packaged in the production artifact.
package com.squareup.anvil.annotations

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
@Repeatable
annotation class ContributesTo(val scope: KClass<*>, val replaces: Array<KClass<*>> = [])

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
@Repeatable
annotation class ContributesBinding(
    val scope: KClass<*>,
    val boundType: KClass<*> = Unit::class,
    val replaces: Array<KClass<*>> = [],
    val ignoreQualifier: Boolean = false,
)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
@Repeatable
annotation class ContributesMultibinding(
    val scope: KClass<*>,
    val boundType: KClass<*> = Unit::class,
    val replaces: Array<KClass<*>> = [],
    val ignoreQualifier: Boolean = false,
)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class ContributesSubcomponent(
    val scope: KClass<*>,
    val parentScope: KClass<*>,
    val modules: Array<KClass<*>> = [],
    val exclude: Array<KClass<*>> = [],
    val replaces: Array<KClass<*>> = [],
)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class MergeComponent(
    val scope: KClass<*>,
    val modules: Array<KClass<*>> = [],
    val dependencies: Array<KClass<*>> = [],
    val exclude: Array<KClass<*>> = [],
)

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.RUNTIME)
annotation class MergeSubcomponent(
    val scope: KClass<*>,
    val modules: Array<KClass<*>> = [],
    val exclude: Array<KClass<*>> = [],
)
