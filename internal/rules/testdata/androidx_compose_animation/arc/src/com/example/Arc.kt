package com.example

import androidx.compose.animation.core.ArcAnimationSpec
import androidx.compose.animation.core.ArcMode
import androidx.compose.animation.core.ExperimentalAnimationSpecApi
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.IntOffset
import androidx.compose.animation.core.ArcAnimationSpec as Arc

@OptIn(ExperimentalAnimationSpecApi::class)
val specs = listOf(
    ArcAnimationSpec<Float>(),
    ArcAnimationSpec<Size>(ArcMode.ArcAbove),
    ArcAnimationSpec<Offset>(),
    ArcAnimationSpec<IntOffset>(),
    ArcAnimationSpec<DpOffset>(),
    ArcAnimationSpec<androidx.compose.ui.geometry.Offset>(),
    androidx.compose.animation.core.ArcAnimationSpec<Int>(),
    Arc<Double>(),
)

@OptIn(ExperimentalAnimationSpecApi::class)
val inferred: ArcAnimationSpec<Float> = ArcAnimationSpec()

typealias Pos = Offset
typealias Scalar = Float

@OptIn(ExperimentalAnimationSpecApi::class)
val aliased = listOf(
    ArcAnimationSpec<Pos>(),
    ArcAnimationSpec<Scalar>(),
)
