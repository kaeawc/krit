package p.props

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

val Modifier.explicitThis: Modifier get() = this.padding(1.dp)
val Modifier.labeledThis: Modifier get() = this@labeledThis.padding(1.dp)
