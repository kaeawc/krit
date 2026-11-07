package test

import android.os.AsyncTask

class MyTask : AsyncTask<Void, Void, String>() {
    override fun doInBackground(vararg params: Void?): String {
        return "result"
    }
}
