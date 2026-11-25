package fixtures.compose

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf

class Cart {
    val items = mutableStateOf(listOf<String>())
    val observable = mutableStateListOf<String>()
    val readOnly: MutableState<List<String>> = mutableStateOf(mutableListOf())
}
