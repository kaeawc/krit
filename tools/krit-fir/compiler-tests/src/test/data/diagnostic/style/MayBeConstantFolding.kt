// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 16, 17, 20, 24
// The compiler's constant evaluator decides, not the written shape of the initializer.
package test.maybeconstantfolding

const val BASE = 10
const val GREETING = "hi"

val <!MayBeConstant!>DOUBLED<!> = BASE * 2
// Go misses the next five: it reads only literals, signs and `+ - * / %` over them, so a template, a property
// of a literal, an infix call, a Java constant and a comparison are not constants to it.
val <!MayBeConstant!>TEMPLATE<!> = "$GREETING there"
val <!MayBeConstant!>LENGTH<!> = "abc".length
val <!MayBeConstant!>SHIFTED<!> = 1 shl 3
val <!MayBeConstant!>MAXIMUM<!> = Int.MAX_VALUE
val <!MayBeConstant!>PARENTHESIZED<!> = (BASE + 1)
val <!MayBeConstant!>LONG_MATH<!> = 2 * 1000L
val <!MayBeConstant!>COMPARED<!> = BASE > 3

val <!MayBeConstant!>FIRST<!> = 1

// Go reports this by resolving `FIRST` in the file, but until `FIRST` is `const` the compiler rejects `const val`
// here (NON_CONST_VAL_USED_IN_CONSTANT_EXPRESSION). The checker waits for the next run, after the edit above.
val SECOND = FIRST + 1
val TEMPLATE_OF_VAL = "$FIRST!"
