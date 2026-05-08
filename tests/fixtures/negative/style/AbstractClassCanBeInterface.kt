package style

abstract class Foo(val x: Int) {
    abstract fun bar()
}

abstract class BasePresenter {
    val scope = Any()
}

abstract class Presenter : BasePresenter {
    abstract fun render()
}
