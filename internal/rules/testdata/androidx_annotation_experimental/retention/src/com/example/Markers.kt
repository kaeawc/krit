package com.example

import kotlin.annotation.AnnotationRetention
import kotlin.annotation.Retention

@RequiresOptIn
@Retention(AnnotationRetention.BINARY)
annotation class GoodKotlin

@RequiresOptIn
annotation class DefaultRetention

@RequiresOptIn(level = RequiresOptIn.Level.WARNING)
@Retention(AnnotationRetention.RUNTIME)
annotation class RuntimeRetention

@RequiresOptIn
@Retention(AnnotationRetention.SOURCE)
annotation class SourceRetention

@androidx.annotation.RequiresOptIn
@Retention(AnnotationRetention.BINARY)
annotation class AndroidxInKotlin

@androidx.annotation.RequiresOptIn
annotation class AndroidxInKotlinDefault

@kotlin.RequiresOptIn
@kotlin.annotation.Retention(AnnotationRetention.BINARY)
annotation class QualifiedGood

@Retention(AnnotationRetention.RUNTIME)
annotation class NotAMarker
