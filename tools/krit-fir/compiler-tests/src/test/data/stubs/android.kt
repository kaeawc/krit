// Compiler-test source stubs; never packaged in the production artifact.
package android

// Java `public final class R` with static nested classes of static final int
// fields: modeled as nested objects so `android.R.string.ok` keeps the Java
// callable id and stays a compile-time constant.
object R {
    object string {
        const val cancel: Int = 17039360
        const val ok: Int = 17039370
    }

    object id {
        const val content: Int = 16908290
    }

    object layout {
        const val simple_list_item_1: Int = 17367043
    }

    object drawable {
        const val ic_menu_add: Int = 17301555
    }

    object color {
        const val black: Int = 17170444
        const val white: Int = 17170443
    }
}

object Manifest {
    object permission {
        const val ACCESS_COARSE_LOCATION: String = "android.permission.ACCESS_COARSE_LOCATION"
        const val ACCESS_FINE_LOCATION: String = "android.permission.ACCESS_FINE_LOCATION"
        const val CAMERA: String = "android.permission.CAMERA"
        const val INTERNET: String = "android.permission.INTERNET"
        const val POST_NOTIFICATIONS: String = "android.permission.POST_NOTIFICATIONS"
        const val READ_CONTACTS: String = "android.permission.READ_CONTACTS"
        const val READ_EXTERNAL_STORAGE: String = "android.permission.READ_EXTERNAL_STORAGE"
        const val RECORD_AUDIO: String = "android.permission.RECORD_AUDIO"
        const val WRITE_EXTERNAL_STORAGE: String = "android.permission.WRITE_EXTERNAL_STORAGE"
    }
}
