// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.tooling.preview

import android.content.res.Configuration

@MustBeDocumented
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.ANNOTATION_CLASS, AnnotationTarget.FUNCTION)
@Repeatable
annotation class Preview(
    val name: String = "",
    val group: String = "",
    val apiLevel: Int = -1,
    val widthDp: Int = -1,
    val heightDp: Int = -1,
    val locale: String = "",
    val fontScale: Float = 1f,
    val showSystemUi: Boolean = false,
    val showBackground: Boolean = false,
    val backgroundColor: Long = 0,
    val uiMode: Int = 0,
    val device: String = Devices.DEFAULT,
    val wallpaper: Int = Wallpapers.NONE,
)

// Multipreview: meta-annotated with two @Preview entries.
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.ANNOTATION_CLASS, AnnotationTarget.FUNCTION)
@Preview(name = "Light")
@Preview(name = "Dark", uiMode = Configuration.UI_MODE_NIGHT_YES or Configuration.UI_MODE_TYPE_NORMAL)
annotation class PreviewLightDark

@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.ANNOTATION_CLASS, AnnotationTarget.FUNCTION)
@Preview(name = "85%", fontScale = 0.85f)
@Preview(name = "100%", fontScale = 1.0f)
@Preview(name = "200%", fontScale = 2f)
annotation class PreviewFontScale

@MustBeDocumented
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.VALUE_PARAMETER)
annotation class PreviewParameter(
    val provider: kotlin.reflect.KClass<out PreviewParameterProvider<*>>,
    val limit: Int = Int.MAX_VALUE,
)

interface PreviewParameterProvider<T> {
    val values: Sequence<T>

    val count: Int
        get() = values.count()
}

object Devices {
    const val DEFAULT: String = ""
    const val PIXEL: String = "id:pixel"
    const val PIXEL_4: String = "id:pixel_4"
    const val TABLET: String = "spec:width=1280dp,height=800dp,dpi=240"
}

object Wallpapers {
    const val NONE: Int = -1
}
