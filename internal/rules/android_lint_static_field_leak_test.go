package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same source.
func TestStaticFieldLeakKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "StaticFieldLeak", `package p

import android.app.Activity
import android.app.Application
import android.content.Context
import android.content.ContextWrapper
import android.os.AsyncTask
import android.view.View
import android.widget.TextView
import java.lang.ref.WeakReference

class Holder(val ctx: Context)
class AppHolder(val appContext: Context)
class Plain(val n: Int)
open class Base { var act: Activity? = null }
class Child : Base()

object Singleton { var s1: Context? = null }
object Quiet { var n: Int = 0 }

var topLevel: View? = null
private val topActivity: Activity? = null

class K {
  val prop = object : AsyncTask<Void, Void, Void>() {
    override fun doInBackground(vararg v: Void?): Void? = null
  }
  inner class Inner : AsyncTask<Void, Void, Void>() {
    override fun doInBackground(vararg v: Void?): Void? = null
  }
  class Nested : AsyncTask<Void, Void, Void>() {
    override fun doInBackground(vararg v: Void?): Void? = null
  }
  fun local() {
    class LocalTask : AsyncTask<Void, Void, Void>() {
      override fun doInBackground(vararg v: Void?): Void? = null
    }
    val t = object : AsyncTask<Void, Void, Void>() {
      override fun doInBackground(vararg v: Void?): Void? = null
    }
  }
  companion object {
    var a1: Context? = null
    val a2: Context? = null
    private var a3: Activity? = null
    lateinit var a4: View
    var a5: Application? = null
    var a6: WeakReference<Context>? = null
    var a7: TextView? = null
    var a8: ContextWrapper? = null
    var a9: Holder? = null
    var a10: AppHolder? = null
    var a11: Plain? = null
    var a12: Child? = null
    var applicationContext1: Context? = null
    var appContext: Context? = null
    var a13: List<Context> = emptyList()
    val a14: Context? get() = null
    val a15: Context by lazy { error("x") }
    @Volatile var a16: Context? = null
    fun stat() {
      val t = object : AsyncTask<Void, Void, Void>() {
        override fun doInBackground(vararg v: Void?): Void? = null
      }
    }
  }
}

fun top() {
  val t = object : AsyncTask<Void, Void, Void>() {
    override fun doInBackground(vararg v: Void?): Void? = null
  }
}
`))
	want := "18:1,18:20,21:1,22:1,25:14,28:15,38:13,43:5,44:5,45:5,46:5,49:5,50:5,51:5,54:5,60:5,62:15"
	if got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestStaticFieldLeakSkipsTestSources(t *testing.T) {
	findings := runRuleByNameOnPath(t, "StaticFieldLeak", "src/test/kotlin/p/T.kt", `package p
import android.content.Context
object Holder { var ctx: Context? = null }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings in test sources, got %d", len(findings))
	}
}
