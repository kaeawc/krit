// Compiler-test source stubs; never packaged in the production artifact.
package android.hardware.camera2

import android.os.Handler
import android.util.AndroidException

object CameraManager {
    val cameraIdList: Array<String>
        get() = TODO()

    fun openCamera(cameraId: String, callback: CameraDevice.StateCallback, handler: Handler?) {
        TODO()
    }
}

abstract class CameraDevice : AutoCloseable {
    abstract val id: String

    abstract override fun close()

    abstract class StateCallback {
        abstract fun onOpened(camera: CameraDevice)

        abstract fun onDisconnected(camera: CameraDevice)

        abstract fun onError(camera: CameraDevice, error: Int)

        open fun onClosed(camera: CameraDevice) {
            TODO()
        }
    }
}

class CameraAccessException(problem: Int) : AndroidException() {
    val reason: Int
        get() = TODO()
}
