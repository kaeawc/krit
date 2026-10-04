// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 14, 18, 40
// A comment inside the comparison does not change what is compared: each
// call below compares a nullable operand with null.
package test

// Both report: Go reads the operator as the comparison's second child and the
// operand as its last, so a comment after the operator does not hide it.
fun commentAfterOperator(c1: String?) {
    <!UseRequireNotNull!>require(c1 != /* c */ null)<!>
}

fun commentAfterOperatorReversed(c2: String?) {
    <!UseRequireNotNull!>require(null != /* c */ c2)<!>
}

fun lineCommentAfterOperator(c4: String?) {
    <!UseRequireNotNull!>require(c4 != // c<!>
        null)
}

// Divergence (Go misses): the comment is the comparison's second child, so Go
// does not see the `!=` operator. c5 is still compared with null.
fun commentBeforeOperator(c5: String?) {
    <!UseRequireNotNull!>require(c5 /* c */ != null)<!>
}

fun commentBeforeOperatorReversed(c6: String?) {
    <!UseRequireNotNull!>require(null /* c */ != c6)<!>
}

// Divergence (Go misses): Go unwraps the parentheses to their first named
// child, which is the comment, not the comparison.
fun commentInParentheses(c7: String?) {
    <!UseRequireNotNull!>require((/* c */ c7 != null))<!>
}

// Both report: the comment follows the comparison.
fun trailingComment(c8: String?) {
    <!UseRequireNotNull!>require(c8 != null /* c */)<!>
}
