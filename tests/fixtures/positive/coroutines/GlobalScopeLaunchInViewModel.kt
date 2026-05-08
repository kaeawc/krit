package test

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

class UserViewModel {
    fun load() {
        GlobalScope.launch {
            fetchData()
        }
    }
}

suspend fun fetchData() {}
