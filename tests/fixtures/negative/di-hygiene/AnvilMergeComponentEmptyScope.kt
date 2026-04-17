package dihygiene

import kotlin.reflect.KClass

annotation class MergeComponent(val scope: KClass<*>)
annotation class ContributesBinding(val scope: KClass<*>)
annotation class ContributesTo(val scope: KClass<*>)

object AppScope

@ContributesTo(AppScope::class)
interface AppApi

@ContributesBinding(AppScope::class)
class AppApiImpl : AppApi

@MergeComponent(AppScope::class)
interface AppComponent
