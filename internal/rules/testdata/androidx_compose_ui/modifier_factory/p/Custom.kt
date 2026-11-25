package p.custom

import androidx.compose.ui.Modifier

class MyElement : Modifier.Element

fun Modifier.custom(): MyElement = MyElement()
fun customNoReceiver() = MyElement()
