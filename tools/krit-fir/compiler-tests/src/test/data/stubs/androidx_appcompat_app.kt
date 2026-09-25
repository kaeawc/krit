// Compiler-test source stubs; never packaged in the production artifact.
package androidx.appcompat.app

import android.app.Dialog
import android.content.Context
import android.content.DialogInterface
import android.view.View
import androidx.fragment.app.FragmentActivity

open class AppCompatActivity : FragmentActivity {
    constructor()

    constructor(contentLayoutId: Int)

    open val supportActionBar: ActionBar?
        get() = TODO()

    override fun setContentView(layoutResID: Int) {
        TODO()
    }

    override fun setContentView(view: View) {
        TODO()
    }

    open fun onSupportNavigateUp(): Boolean = TODO()
}

abstract class ActionBar {
    abstract var title: CharSequence?

    abstract fun setDisplayHomeAsUpEnabled(showHomeAsUp: Boolean)

    abstract fun show()

    abstract fun hide()
}

open class AlertDialog(context: Context) : Dialog(context) {
    open class Builder(context: Context) {
        open fun setTitle(title: CharSequence?): Builder = TODO()

        open fun setTitle(titleId: Int): Builder = TODO()

        open fun setMessage(message: CharSequence?): Builder = TODO()

        open fun setMessage(messageId: Int): Builder = TODO()

        open fun setPositiveButton(text: CharSequence?, listener: DialogInterface.OnClickListener?): Builder = TODO()

        open fun setPositiveButton(textId: Int, listener: DialogInterface.OnClickListener?): Builder = TODO()

        open fun setNegativeButton(text: CharSequence?, listener: DialogInterface.OnClickListener?): Builder = TODO()

        open fun setNegativeButton(textId: Int, listener: DialogInterface.OnClickListener?): Builder = TODO()

        open fun setCancelable(cancelable: Boolean): Builder = TODO()

        open fun setView(view: View?): Builder = TODO()

        open fun create(): AlertDialog = TODO()

        open fun show(): AlertDialog = TODO()
    }
}
