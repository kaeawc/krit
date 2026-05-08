package test

class LocationActivity {
    fun requestBackground() {
        requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), 100)
    }
}
