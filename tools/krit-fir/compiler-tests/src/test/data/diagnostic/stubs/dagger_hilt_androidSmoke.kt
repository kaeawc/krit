// Smoke: Hilt entry points on an Application and an Activity.
package stubs

import android.app.Application
import androidx.appcompat.app.AppCompatActivity
import dagger.hilt.android.AndroidEntryPoint
import dagger.hilt.android.HiltAndroidApp
import javax.inject.Inject

@HiltAndroidApp
class HiltSmokeApp : Application()

@AndroidEntryPoint
class HiltSmokeActivity : AppCompatActivity() {
    @Inject
    lateinit var clock: HiltClock
}

class HiltClock @Inject constructor()
