package project.lib

// Called from //project/app: a per-target action over //project/lib alone
// cannot see that and would report it as dead code.
fun usedApi(x: Int): Int = x + 1

// Called from nowhere: the project report finds it.
fun unusedApi(x: Int): Int = x * 2
