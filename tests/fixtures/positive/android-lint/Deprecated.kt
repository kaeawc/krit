package test

class MyTask : AsyncTask<Void, Void, String>() {
    override fun doInBackground(vararg params: Void?): String {
        return "result"
    }
}
