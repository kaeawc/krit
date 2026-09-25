// Smoke: @Parcelize data class with @IgnoredOnParcel and a custom Parceler.
package stubs

import android.os.Parcel
import android.os.Parcelable
import kotlinx.parcelize.IgnoredOnParcel
import kotlinx.parcelize.Parceler
import kotlinx.parcelize.Parcelize
import kotlinx.parcelize.RawValue

@Parcelize
data class ParcelUser(val id: Long, val name: String, val extra: @RawValue Any?) : Parcelable {
    @IgnoredOnParcel
    val display: String = "$id $name"
}

object ParcelUserParceler : Parceler<ParcelUser> {
    override fun create(parcel: Parcel): ParcelUser = ParcelUser(parcel.readLong(), parcel.readString() ?: "", null)

    override fun ParcelUser.write(parcel: Parcel, flags: Int) {
        parcel.writeLong(id)
        parcel.writeString(name)
    }
}
