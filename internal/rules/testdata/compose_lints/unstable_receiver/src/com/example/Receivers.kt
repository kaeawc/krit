package com.example

import androidx.compose.animation.AnimatedVisibilityScope
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.lazy.LazyItemScope
import androidx.compose.runtime.Composable
import com.example.model.Derived
import com.example.model.Frozen
import com.example.model.Id
import com.example.model.Kind
import com.example.model.Leaf
import com.example.model.Marked
import com.example.model.Plain

@Composable fun Plain.A() {}
@Composable fun Derived.B() {}
@Composable fun Marked.C() {}
@Composable fun Frozen.D() {}
@Composable fun Kind.E() {}
@Composable fun Id.F() {}
@Composable fun Leaf.G() {}
@Composable fun RowScope.H() {}
@Composable fun ColumnScope.I() {}
@Composable fun LazyItemScope.J() {}
@Composable fun AnimatedVisibilityScope.K() {}
@Composable fun String.L() {}
@Composable fun Int.M() {}
@Composable fun Int?.N() {}
@Composable fun Any.O() {}
@Composable fun List<String>.P() {}
@Composable fun <T> T.Q() {}
@Composable fun (() -> Unit).R() {}
@Composable fun Plain?.S() {}
@Composable fun Plain.ValueReturning(): Int = 1

class Screen {
    @Composable fun Content() {}
    @Composable fun Value(): Int = 1
    companion object {
        @Composable fun Companion() {}
    }
}

object Singleton {
    @Composable fun Content() {}
}

interface Contract {
    @Composable fun Render()
}

enum class Mode {
    X;
    @Composable fun Render() {}
}

@Composable fun TopLevel() {}
