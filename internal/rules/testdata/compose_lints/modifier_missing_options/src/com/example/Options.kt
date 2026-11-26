package com.example

import androidx.compose.runtime.Composable

@Composable
fun PublicCustom() {
    MyCard()
}

@Composable
internal fun InternalCustom() {
    MyCard()
}

@Composable
private fun PrivateCustom() {
    MyCard()
}

@Composable
fun NotListed() {
    Other()
}

@Composable
fun MyCard() {}

@Composable
fun Other() {}
