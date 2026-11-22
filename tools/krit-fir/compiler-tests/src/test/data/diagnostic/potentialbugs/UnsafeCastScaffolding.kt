// go-lines: 32, 35, 38, 45, 47, 50
package unsafecastscaffolding

import androidx.compose.runtime.Composable

annotation class Preview
annotation class OrdinaryCastAnnotation
annotation class ComposeScaffolding

@Preview
annotation class ThemeSet

annotation class PreviewThemes

@PreviewThemes
annotation class Themes

fun previewImpossible() = 1 as String
fun sampleImpossible() = 1 as String
fun fakeImpossible() = 1 as String
fun mockImpossible() = 1 as String
fun stubImpossible() = 1 as String
fun fixtureImpossible() = 1 as String
fun impossiblePreview() = 1 as String
fun impossibleSample() = 1 as String
fun impossibleFixture() = 1 as String

@Preview
fun annotatedPreview() = 1 as String

@OrdinaryCastAnnotation
fun annotatedProduction() = <!UnsafeCast!>1 as String<!>

@ComposeScaffolding
fun composedAnnotationDoesNotHideImpossibleCast() = <!UnsafeCast!>1 as String<!>

@Composable
fun composableDoesNotHideImpossibleCast() = <!UnsafeCast!>1 as String<!>

@ThemeSet
fun localMultipreviewIsScaffolding() = 1 as String

// FIR's wildcard accepts the Preview* prefix; Go's multipreview matcher accepts only androidx names or a *Preview suffix.
@Themes
fun pluralMultipreviewIsScaffolding() = 1 as String

fun production() = <!UnsafeCast!>1 as String<!>

fun previewOuter() {
    fun productionInner() = <!UnsafeCast!>1 as String<!>
}
