// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.platform

import android.content.Context
import androidx.compose.runtime.ProvidableCompositionLocal
import androidx.compose.runtime.staticCompositionLocalOf

val LocalContext: ProvidableCompositionLocal<Context> = staticCompositionLocalOf { TODO() }

val LocalInspectionMode: ProvidableCompositionLocal<Boolean> = staticCompositionLocalOf { false }
