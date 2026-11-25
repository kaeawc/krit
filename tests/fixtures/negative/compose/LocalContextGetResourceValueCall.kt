package fixtures.compose

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource

@Composable
fun Title(): String = stringResource(android.R.string.ok)

@Composable
fun PackageName(): String = LocalContext.current.packageName

fun titleOf(context: Context): String = context.getString(android.R.string.ok)
