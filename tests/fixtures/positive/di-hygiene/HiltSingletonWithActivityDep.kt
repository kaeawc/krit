package dihygiene

annotation class Singleton
annotation class Inject

class Activity
interface Navigator

@Singleton
class NavigatorImpl @Inject constructor(val activity: Activity) : Navigator
