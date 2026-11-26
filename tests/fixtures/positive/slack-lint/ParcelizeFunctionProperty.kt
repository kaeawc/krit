package fixtures.positive.slacklint

import android.os.Parcelable
import kotlinx.parcelize.Parcelize

@Parcelize
class Action(val label: String, val onClick: () -> Unit) : Parcelable
