package test

class Singleton {
    private var instance: Singleton? = null

    fun getInstance(): Singleton {
        if (instance == null) {
            synchronized(this) {
                if (instance == null) {
                    instance = Singleton()
                }
            }
        }
        return instance!!
    }
}
