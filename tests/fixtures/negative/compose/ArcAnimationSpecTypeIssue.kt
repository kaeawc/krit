package fixtures.compose

import androidx.compose.animation.core.ArcAnimationSpec
import androidx.compose.animation.core.ExperimentalAnimationSpecApi
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.unit.IntOffset

@OptIn(ExperimentalAnimationSpecApi::class)
val specs = listOf(
    ArcAnimationSpec<Offset>(),
    ArcAnimationSpec<IntOffset>(),
    ArcAnimationSpec<androidx.compose.ui.unit.DpOffset>(),
)

@OptIn(ExperimentalAnimationSpecApi::class)
val inferred: ArcAnimationSpec<Float> = ArcAnimationSpec()
