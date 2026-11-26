package com.example

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionLayout
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.Card
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

const val KEY = "const"

@Composable
fun Item(text: String, modifier: Modifier = Modifier) {
    Text(text, modifier)
}

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Scopes() {
    SharedTransitionScope { Box {} }
    SharedTransitionScope { modifier -> Box {} }
    SharedTransitionScope { modifier -> Box(modifier) {} }
    SharedTransitionScope { Box(it) {} }
    SharedTransitionScope { _ -> Box {} }
}

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Lists(names: List<String>, visible: Boolean) {
    SharedTransitionLayout {
        AnimatedVisibility(visible) {
            LazyColumn {
                items(names) { name ->
                    Item(name, Modifier.sharedElement(rememberSharedContentState("key"), this@AnimatedVisibility))
                    Item(name, Modifier.sharedBounds(rememberSharedContentState(key = KEY), this@AnimatedVisibility))
                    Item(name, Modifier.sharedElement(rememberSharedContentState("a" + "b"), this@AnimatedVisibility))
                    Item(name, Modifier.sharedElement(rememberSharedContentState(1), this@AnimatedVisibility))
                    Item(name, Modifier.sharedElement(rememberSharedContentState(name), this@AnimatedVisibility))
                    Item(name, Modifier.sharedElement(rememberSharedContentState("k$name"), this@AnimatedVisibility))
                    Text(name, Modifier.sharedElement(rememberSharedContentState("text"), this@AnimatedVisibility))
                    Item(name, Modifier.sharedElement(rememberSharedContentState("chained"), this@AnimatedVisibility).then(Modifier))
                    Item(name, modifier = Modifier.then(Modifier).sharedElement(rememberSharedContentState("named"), this@AnimatedVisibility))
                }
                itemsIndexed(names) { index, name ->
                    Item(name, Modifier.sharedElement(rememberSharedContentState("indexed"), this@AnimatedVisibility))
                }
                item {
                    Item("x", Modifier.sharedElement(rememberSharedContentState("single"), this@AnimatedVisibility))
                }
            }
            Item("x", Modifier.sharedElement(rememberSharedContentState("outside"), this@AnimatedVisibility))
        }
    }
}

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun TrailingLambdas(names: List<String>, visible: Boolean) {
    SharedTransitionLayout {
        AnimatedVisibility(visible) {
            LazyColumn {
                items(names) { name ->
                    Box(Modifier.sharedElement(rememberSharedContentState("boxkey"), this@AnimatedVisibility)) { Text(name) }
                    Card(modifier = Modifier.sharedBounds(rememberSharedContentState(key = "card"), this@AnimatedVisibility)) { Text(name) }
                    Box(Modifier.sharedElement(rememberSharedContentState(name), this@AnimatedVisibility)) { Text(name) }
                    Box {
                        Item(name, Modifier.sharedElement(rememberSharedContentState("nested"), this@AnimatedVisibility))
                    }
                    val local = "local"
                    Item(name, Modifier.sharedElement(rememberSharedContentState(local), this@AnimatedVisibility))
                    var mutable = "mutable"
                    Item(name, Modifier.sharedElement(rememberSharedContentState(mutable), this@AnimatedVisibility))
                }
            }
        }
    }
}

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun PositionalScope() {
    SharedTransitionScope({ modifier -> Box {} })
}
