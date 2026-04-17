package dihygiene

import kotlin.reflect.KClass

annotation class ContributesBinding(val scope: KClass<*>)
annotation class ContributesTo(val scope: KClass<*>)
annotation class Inject

object AppScope
object FeatureScope

@ContributesTo(FeatureScope::class)
interface FeatureApi

@ContributesBinding(AppScope::class)
class FeatureImpl @Inject constructor() : FeatureApi
