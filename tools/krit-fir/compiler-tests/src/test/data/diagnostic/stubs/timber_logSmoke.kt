// Smoke: Timber.Forest static-style calls, tag(), and planting a Tree subclass.
package stubs

import android.util.Log
import timber.log.Timber

class CrashReportingTree : Timber.Tree() {
    override fun isLoggable(tag: String?, priority: Int): Boolean = priority >= Log.WARN

    override fun log(priority: Int, tag: String?, message: String, t: Throwable?) {
        println("$priority $tag $message $t")
    }
}

fun timberSmoke(error: Throwable) {
    Timber.plant(Timber.DebugTree())
    Timber.plant(CrashReportingTree())
    Timber.d("debug")
    Timber.d("formatted %s", 1)
    Timber.i("info")
    Timber.w(error)
    Timber.w(error, "warn %d", 2)
    Timber.e(error, "error")
    Timber.v("verbose")
    Timber.wtf("wtf")
    Timber.tag("Tag").d("tagged")
    Timber.log(Log.INFO, "logged")
    val tree: Timber.Tree = Timber.asTree()
    tree.i("via tree")
    println(Timber.treeCount)
    Timber.uprootAll()
}
