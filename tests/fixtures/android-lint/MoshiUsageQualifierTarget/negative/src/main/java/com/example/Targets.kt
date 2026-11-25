package com.example

import com.squareup.moshi.JsonQualifier

// A missing @Target includes FIELD.
@JsonQualifier
annotation class DefaultTarget

@JsonQualifier
@Target(AnnotationTarget.FIELD, AnnotationTarget.FUNCTION)
annotation class WithField

// Not a JsonQualifier.
@Target(AnnotationTarget.CLASS)
annotation class Plain
