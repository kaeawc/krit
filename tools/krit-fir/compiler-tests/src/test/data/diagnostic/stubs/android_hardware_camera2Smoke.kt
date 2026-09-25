// Smoke: open a camera through CameraManager with a StateCallback subclass.
package stubs

import android.content.Context
import android.hardware.camera2.CameraAccessException
import android.hardware.camera2.CameraDevice
import android.hardware.camera2.CameraManager
import android.os.Handler
import android.os.Looper

@Suppress("MissingPermission")
fun openFirstCamera(context: Context) {
    val manager = context.getSystemService(Context.CAMERA_SERVICE) as CameraManager
    try {
        val id = manager.cameraIdList.first()
        manager.openCamera(
            id,
            object : CameraDevice.StateCallback() {
                override fun onOpened(camera: CameraDevice) {
                    camera.close()
                }

                override fun onDisconnected(camera: CameraDevice) {}

                override fun onError(camera: CameraDevice, error: Int) {}
            },
            Handler(Looper.getMainLooper()),
        )
    } catch (e: CameraAccessException) {
        println(e.reason)
    }
}
