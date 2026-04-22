package test

import android.Manifest
import android.hardware.Camera
import android.location.LocationManager
import android.media.MediaRecorder
import androidx.core.content.ContextCompat

class LocationTracker {
    fun startTracking(manager: LocationManager) {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == GRANTED) {
            manager.requestLocationUpdates("gps", 1000L, 0f, listener)
        }
    }

    fun openCamera() {
        if (ContextCompat.checkSelfPermission(context, "android.permission.CAMERA") == GRANTED) {
            Camera.open()
        }
    }

    fun openCameraAfterReturn() {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) != GRANTED) {
            return
        }
        Camera.open()
    }

    fun record(recorder: MediaRecorder) {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) == GRANTED) {
            recorder.setAudioSource(MediaRecorder.AudioSource.MIC)
        }
    }

    fun commentsAndStringsOnly() {
        // Camera.open()
        val text = "checkSelfPermission requestLocationUpdates"
    }
}

class Fake {
    fun requestLocationUpdates() {}
}

fun localSameName(fake: Fake) {
    fake.requestLocationUpdates()
}
