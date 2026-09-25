// Smoke: SingletonComponent as a class-literal install target.
package stubs

import dagger.hilt.components.SingletonComponent
import kotlin.reflect.KClass

val SingletonTarget: KClass<SingletonComponent> = SingletonComponent::class
