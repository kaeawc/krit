package fixtures.compose

import androidx.compose.runtime.mutableStateOf

class Cart {
    val items = mutableStateOf(mutableListOf<String>())
}
