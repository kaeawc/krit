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
