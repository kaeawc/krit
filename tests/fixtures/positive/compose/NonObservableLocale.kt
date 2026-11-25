package fixtures.compose

import androidx.compose.runtime.Composable
import java.util.Locale

@Composable
fun Language(): String = Locale.getDefault().language
