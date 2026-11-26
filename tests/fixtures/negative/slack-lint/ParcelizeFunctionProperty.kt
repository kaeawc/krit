package fixtures.negative.slacklint

import android.os.Parcelable
import kotlinx.parcelize.Parcelize

@Parcelize
class Action(val label: String, val ids: List<Int>) : Parcelable

// Not @Parcelize.
class Handler(val onClick: () -> Unit)
