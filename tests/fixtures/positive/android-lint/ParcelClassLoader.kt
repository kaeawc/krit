package com.example

import android.os.Bundle
import android.os.Parcel

fun read(parcel: Parcel): Bundle? {
    return parcel.readBundle()
}
