// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.parcelize

import android.os.Parcel

// The parcelize compiler plugin is not loaded in compiler-tests, so @Parcelize
// classes compile only because android.os.Parcelable's members carry default
// bodies in these stubs.
@Target(AnnotationTarget.CLASS)
@Retention(AnnotationRetention.BINARY)
annotation class Parcelize

@Target(AnnotationTarget.PROPERTY)
@Retention(AnnotationRetention.SOURCE)
annotation class IgnoredOnParcel

@Target(AnnotationTarget.TYPE)
@Retention(AnnotationRetention.BINARY)
annotation class RawValue

interface Parceler<T> {
    fun create(parcel: Parcel): T

    fun T.write(parcel: Parcel, flags: Int)

    fun newArray(size: Int): Array<T> = TODO()
}

@Retention(AnnotationRetention.SOURCE)
@Repeatable
@Target(AnnotationTarget.CLASS, AnnotationTarget.PROPERTY)
annotation class TypeParceler<T, P : Parceler<in T>>

@Retention(AnnotationRetention.SOURCE)
@Target(AnnotationTarget.TYPE)
annotation class WriteWith<P : Parceler<*>>
