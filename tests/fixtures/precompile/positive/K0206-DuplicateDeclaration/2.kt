// EXPECTED-KOTLINC-ERROR: REDECLARATION
// Two top-level classes with identical name.
class Holder(val n: Int)

class Holder(val s: String)
