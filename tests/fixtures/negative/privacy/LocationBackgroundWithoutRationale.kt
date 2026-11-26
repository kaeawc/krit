package test

class LocationActivity {
    fun requestBackground() {
        if (shouldShowRequestPermissionRationale(Manifest.permission.ACCESS_BACKGROUND_LOCATION)) {
            showRationaleDialog()
        }
        requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), 100)
    }

    fun showRationaleDialog() {}
}

// Self-referencing and mutually-referencing initializers must not recurse
// forever while resolving the requested permission (#1611).
class PermissionHolder(permissions: Array<String>) {
    val permissions = permissions
    val first: Array<String> get() = second
    val second: Array<String> get() = first

    fun request() {
        requestPermissions(permissions, 101)
        requestPermissions(first, 102)
    }
}
