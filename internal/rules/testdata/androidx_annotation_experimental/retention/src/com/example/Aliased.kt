package com.example

import androidx.annotation.RequiresOptIn as AndroidxRequiresOptIn
import kotlin.annotation.Retention as Keep

@AndroidxRequiresOptIn
@Keep(AnnotationRetention.BINARY)
annotation class AliasedMarker
