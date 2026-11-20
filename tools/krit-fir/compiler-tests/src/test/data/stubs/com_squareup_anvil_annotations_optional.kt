package com.squareup.anvil.annotations.optional

import kotlin.reflect.KClass

@Target(AnnotationTarget.CLASS)
annotation class SingleIn(val scope: KClass<*>)
