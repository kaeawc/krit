package com.example

import kotlin.annotation.AnnotationRetention.BINARY as B
import kotlin.annotation.AnnotationRetention.SOURCE as S

@RequiresOptIn
@Retention(B)
annotation class AliasedBinary

@RequiresOptIn
@Retention(S)
annotation class AliasedSource
