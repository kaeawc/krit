package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

class UserViewModel(private val viewModelScope: CoroutineScope) {
    fun load() {
        viewModelScope.launch {
            fetchData()
        }
    }
}

suspend fun fetchData() {}
