package com.example

import com.squareup.moshi.JsonQualifier

@JsonQualifier
@Target(AnnotationTarget.PROPERTY, AnnotationTarget.VALUE_PARAMETER)
annotation class NoField

@JsonQualifier
@kotlin.annotation.Target(AnnotationTarget.CLASS)
annotation class QualifiedTarget
