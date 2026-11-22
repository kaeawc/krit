package stubs

import androidx.compose.runtime.snapshots.*

fun snapshotSmoke(list: SnapshotStateList<String>, map: SnapshotStateMap<String, String>) {
    list.add("entry")
    map["key"] = list[0]
}

fun snapshotConstructors(): StateObject = SnapshotStateList<String>().also {
    val randomAccess: java.util.RandomAccess = it
    val mutable: MutableList<String> = it
    mutable.add(randomAccess.toString())
    val map: StateObject = SnapshotStateMap<String, String>()
    map.prependStateRecord(it.firstStateRecord)
}
