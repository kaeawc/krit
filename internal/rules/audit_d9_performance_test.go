package rules_test

import "testing"

func TestAuditPerformance(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"foreach on mapped range", "ForEachOnRange", `
package test
fun f() { (1..10).map { it * 2 }.forEach { println(it) } }
`, 1}, // detekt looks at the root of the receiver chain only
		{"foreach on stepped range", "ForEachOnRange", `
package test
fun f() { (1..10).step(2).forEach { println(it) } }
`, 1},
		{"array nullable int", "ArrayPrimitive", `
package test
fun f(a: Array<Int?>) {}
`, 0},
		{"array int", "ArrayPrimitive", `
package test
fun f(a: Array<Int>) {}
`, 1},
		{"array int override", "ArrayPrimitive", `
package test
interface I { fun f(a: Array<Int>) }
class C : I { override fun f(a: Array<Int>) {} }
`, 1},
		{"init hex d", "UnnecessaryInitOnArray", `
package test
val a = IntArray(3) { 0x0D }
`, 0},
		{"init zero", "UnnecessaryInitOnArray", `
package test
val a = IntArray(3) { 0 }
`, 1},
		{"cast needed any to string", "UnnecessaryTypeCasting", `
package test
fun f(o: Any) { val s: String = o as String; println(s) }
`, 0},
		{"cast nullable to nonnull", "UnnecessaryTypeCasting", `
package test
fun f(o: String?) { val s = o as String; println(s) }
`, 0},
		{"cast same type", "UnnecessaryTypeCasting", `
package test
fun f(o: String) { val s = o as String; println(s) }
`, 1},
		{"temp instantiation parse is not a constructor", "UnnecessaryTemporaryInstantiation", `
package test
fun f(s: String) = Integer.parseInt(s).toString()
`, 0},
		{"temp instantiation valueof is not a constructor", "UnnecessaryTemporaryInstantiation", `
package test
fun f(n: Int) = Integer.valueOf(n).toString()
`, 0},
		{"temp instantiation constructor", "UnnecessaryTemporaryInstantiation", `
package test
fun f(n: Int) = Integer(n).toString()
`, 1},
		{"binary expr", "UnnecessaryPartOfBinaryExpression", `
package test
fun f(a: Boolean) = a && true
`, 1},
		{"bitmap decode resource", "BitmapDecodeWithoutOptions", `
package test
import android.graphics.BitmapFactory
fun f(res: android.content.res.Resources) = BitmapFactory.decodeResource(res, 1)
`, 1},
		{"bitmap decode file with options", "BitmapDecodeWithoutOptions", `
package test
import android.graphics.BitmapFactory
fun f(p: String, o: BitmapFactory.Options) = BitmapFactory.decodeFile(p, o)
`, 0},
		{"spread of vararg param", "SpreadOperator", `
package test
fun g(vararg x: Int) {}
fun f(vararg x: Int) { g(*x) }
`, 0},
		{"spread of array", "SpreadOperator", `
package test
fun g(vararg x: Int) {}
fun f(a: IntArray) { g(*a) }
`, 1},
	})
}
