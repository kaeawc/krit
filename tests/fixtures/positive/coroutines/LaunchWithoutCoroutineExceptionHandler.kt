package test

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

fun start() {
    GlobalScope.launch {
        throw RuntimeException("boom")
    }
}
