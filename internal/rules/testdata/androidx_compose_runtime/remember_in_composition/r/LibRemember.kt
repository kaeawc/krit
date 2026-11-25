package r

import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.border.BorderLogic
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.style.MutableStyleState
import androidx.compose.foundation.text.input.TextFieldState
import androidx.compose.material3.RangeSliderState
import androidx.compose.material3.SliderState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.movableContentWithReceiverOf
import androidx.compose.ui.graphics.Color

@Composable
fun LibRemember(src: MutableInteractionSource) {
    BorderLogic()
    BringIntoViewRequester()
    MutableStyleState(src)
    TextFieldState()
    RangeSliderState()
    SliderState(0.5f)
    movableContentWithReceiverOf<Int> { }
    Animatable(0f)
    androidx.compose.animation.Animatable(Color.Red)
}

class MutableInteractionSourceHolder {
    fun MutableInteractionSource(): Int = 1

    @Composable
    fun Shadowed() {
        MutableInteractionSource()
    }
}
