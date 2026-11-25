package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ParentDataModifier
import androidx.compose.ui.unit.Density

fun Modifier.marker(): Modifier = then(object : Modifier.Element {})

fun Modifier.optional(): Modifier? = this

fun Modifier.parentData(): ParentDataModifier = object : ParentDataModifier {
    override fun Density.modifyParentData(parentData: Any?): Any? = parentData
}
