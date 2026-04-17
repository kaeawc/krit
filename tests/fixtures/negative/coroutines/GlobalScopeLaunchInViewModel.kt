package test

import kotlinx.coroutines.CoroutineScope

class UserViewModel(private val viewModelScope: CoroutineScope) {
    fun load() {
        viewModelScope.launch {
            fetchData()
        }
    }
}

suspend fun fetchData() {}
