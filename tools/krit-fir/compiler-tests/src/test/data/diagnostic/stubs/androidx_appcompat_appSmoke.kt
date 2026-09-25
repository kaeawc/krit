// Smoke: AppCompatActivity lifecycle override, its supertype chain, and AlertDialog.
package stubs

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.LifecycleOwner

class SmokeCompatActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(android.R.layout.simple_list_item_1)
        supportActionBar?.setDisplayHomeAsUpEnabled(true)
        supportActionBar?.title = "Smoke"
        supportFragmentManager.beginTransaction().commit()
    }

    override fun onSupportNavigateUp(): Boolean = super.onSupportNavigateUp()
}

fun hierarchy(activity: AppCompatActivity) {
    val owner: LifecycleOwner = activity
    val component: ComponentActivity = activity
    val fragmentActivity: FragmentActivity = activity
    val context: Context = activity
    println("$owner $component $fragmentActivity $context")
}

fun confirm(context: Context) {
    AlertDialog.Builder(context)
        .setTitle("Delete?")
        .setMessage("This cannot be undone")
        .setCancelable(false)
        .setPositiveButton("Delete") { dialog, _ -> dialog.dismiss() }
        .setNegativeButton(android.R.string.cancel, null)
        .show()
}
