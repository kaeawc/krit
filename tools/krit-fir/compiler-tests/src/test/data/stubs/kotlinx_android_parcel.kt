// Compiler-test source stubs; never packaged in the production artifact.
// The legacy kotlin-android-extensions parcel annotations (superseded by
// kotlinx.parcelize).
package kotlinx.android.parcel

@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class Parcelize

@Target(AnnotationTarget.PROPERTY)
@Retention(AnnotationRetention.SOURCE)
annotation class IgnoredOnParcel

@Target(AnnotationTarget.TYPE)
@Retention(AnnotationRetention.BINARY)
annotation class RawValue
