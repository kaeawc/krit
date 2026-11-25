package p.then

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Modifier.bad(): Modifier = this.then(padding(1.dp))
fun Modifier.badUnqualified(): Modifier = then(padding(1.dp))
fun Modifier.badCustom(): Modifier = then(custom())
fun Modifier.good(): Modifier = this.then(Modifier.padding(1.dp))
fun Modifier.goodChain(): Modifier = then(Modifier.custom().padding(2.dp))
fun Modifier.nested(): Modifier = then(Modifier.padding(1.dp).then(padding(2.dp)))
fun Modifier.custom(): Modifier = this
fun outside(m: Modifier): Modifier = m.then(Modifier.padding(1.dp))
fun Modifier.lambdaReceiver(): Modifier = then(with(Modifier) { padding(1.dp) })
