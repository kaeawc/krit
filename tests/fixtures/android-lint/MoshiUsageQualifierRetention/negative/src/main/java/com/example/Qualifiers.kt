package com.example

import com.squareup.moshi.JsonQualifier

// Kotlin's default retention is RUNTIME.
@JsonQualifier
annotation class DefaultRetention

@JsonQualifier
@Retention(AnnotationRetention.RUNTIME)
annotation class RuntimeRetention

// Not a JsonQualifier.
@Retention(AnnotationRetention.SOURCE)
annotation class Plain
