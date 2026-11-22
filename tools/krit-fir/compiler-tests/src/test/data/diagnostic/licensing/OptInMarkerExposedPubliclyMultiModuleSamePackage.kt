// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package com.example.api

@kotlin.RequiresOptIn
annotation class ExperimentalCoroutinesApi

// Core and feature are separate Gradle modules with the same package. FIR
// skips this marker within its package tree; Go reports it by name.
@ExperimentalCoroutinesApi // FIR abstains: skips this marker within its package tree; Go reports it by name.
fun featureApi() = Unit
