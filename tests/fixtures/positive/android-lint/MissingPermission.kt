package test

import android.hardware.Camera
import android.location.LocationManager
import android.media.MediaRecorder

class LocationTracker {
    fun startTracking(manager: LocationManager) {
        manager.requestLocationUpdates(
            "gps",
            1000L,
            0f,
            listener
        )
    }

    fun lastLocation(manager: LocationManager) {
        manager.getLastKnownLocation("gps")
    }

    fun openCamera() {
        Camera.open()
    }

    fun record(recorder: MediaRecorder) {
        recorder.setAudioSource(MediaRecorder.AudioSource.MIC)
    }

    fun requestThenRecord(recorder: MediaRecorder) {
        requestPermissions(arrayOf(android.Manifest.permission.RECORD_AUDIO), 1)
        recorder.setAudioSource(MediaRecorder.AudioSource.MIC)
    }

    fun openCameraInDeniedBranch() {
        if (checkSelfPermission(android.Manifest.permission.CAMERA) == PERMISSION_GRANTED) {
            println("safe branch")
        } else {
            Camera.open()
        }
    }
}
