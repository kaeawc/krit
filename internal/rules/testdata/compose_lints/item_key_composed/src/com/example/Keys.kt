package com.example

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key

data class Item(val id: String, val name: String)

@Composable
fun Items(list: List<Item>) {
    LazyColumn {
        items(list, key = { it.hashCode() }) { item -> }
        items(list, key = { item -> item.id }) { item -> }
        item(key = list.hashCode()) {}
        items(list.size, key = { index -> list[index].name.hashCode() + index }) {}
        items(list, key = { hashCode() }) {}
        items(list, key = { it.hashCode(1) }) {}
    }
    key(list.hashCode()) {}
    val keyed = mapOf("key" to list.hashCode())
    println(keyed)
}

fun LazyListScope.notComposable(list: List<Item>) {
    items(list, key = { it.hashCode() }) {}
}

fun Item.hashCode(seed: Int): Int = seed
