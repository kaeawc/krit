package fixtures.negative.potentialbugs

class AvoidReferentialEquality {
    fun check(a: String, b: String): Boolean {
        return a == b
    }

    fun sentinel(d: Any): Boolean {
        return d === DISPOSED
    }

    fun unchanged(unwrapped: TreeNode, current: TreeNode): Boolean {
        return unwrapped !== current
    }
}

object DISPOSED
class TreeNode
