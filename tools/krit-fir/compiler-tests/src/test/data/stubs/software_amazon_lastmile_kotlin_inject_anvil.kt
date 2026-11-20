package software.amazon.lastmile.kotlin.inject.anvil

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
annotation class SingleIn(val scope: KClass<*>)

@Target(AnnotationTarget.CLASS)
annotation class ContributesBinding(val scope: KClass<*>)
