package dihygiene

annotation class Singleton
annotation class ActivityScoped
annotation class Inject

class Activity
class Context
interface Navigator

// ActivityScoped + Activity dep: scopes match.
@ActivityScoped
class NavigatorImpl @Inject constructor(val activity: Activity) : Navigator

// @Singleton with application-scoped dependency only.
@Singleton
class AppRepo @Inject constructor(val context: Context)

// No DI scope annotation; not flagged.
class FreeForm(val activity: Activity)
