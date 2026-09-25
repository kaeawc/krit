// Smoke: the deprecated synthetic-parcel @Parcelize on a Parcelable class.
package stubs

import android.os.Parcelable
import kotlinx.android.parcel.IgnoredOnParcel
import kotlinx.android.parcel.Parcelize

@Parcelize
data class LegacyParcel(val id: Int) : Parcelable {
    @IgnoredOnParcel
    val derived: String = "$id"
}
