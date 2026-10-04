// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 22, 39
// Go findings FIR drops because the code lacks what the message asserts.
package test

import java.io.Serializable

// Go reports this because the backticked name's text is not serialVersionUID;
// FIR is correct to drop it because the companion declares serialVersionUID,
// so the class is not missing one.
class Backticked : Serializable {
    companion object {
        private const val `serialVersionUID` = 1L
    }
}

abstract class Ranked : Comparable<Serializable>

// Go reports this because Ranked's supertype list names Serializable in a
// type argument; FIR is correct to drop it because Ranked is only
// Comparable, so the class is not Serializable.
class Ranking : Ranked() {
    override fun compareTo(other: Serializable): Int = 0
}

open class Node

object Tree {
    open class Node : Serializable {
        companion object {
            private const val serialVersionUID = 1L
        }
    }
}

// Go reports this because it resolves `Node` by simple name to Tree.Node,
// which is Serializable; FIR is correct to drop it because the class extends
// the top-level Node, which is not Serializable.
class Leaf : Node()
