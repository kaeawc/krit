package com.example

import com.squareup.moshi.JsonQualifier

@JsonQualifier
@Retention(AnnotationRetention.BINARY)
annotation class BinaryRetention

@JsonQualifier
@Retention(AnnotationRetention.SOURCE)
annotation class SourceRetention
