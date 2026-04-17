package rules_test

import "testing"

func TestOverrideAbstract(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "OverrideAbstract", `
package test

class MyService : Service() {
    // missing onBind
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "OverrideAbstract", `
package test

class MyService : Service() {
    override fun onBind(intent: Intent): IBinder? = null
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestParcelCreator(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ParcelCreator", `
package test

class MyData : Parcelable {
    val name: String = ""
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ParcelCreator", `
package test

@Parcelize
class MyData : Parcelable {
    val name: String = ""
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSwitchIntDef(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "SwitchIntDef", `
package test

fun check(visibility: Int) {
    when (visibility) {
        VISIBLE -> show()
        GONE -> hide()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "SwitchIntDef", `
package test

fun check(visibility: Int) {
    when (visibility) {
        VISIBLE -> show()
        INVISIBLE -> dim()
        GONE -> hide()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestTextViewEdits(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "TextViewEdits", `
package test

fun update() {
    editTextField.setText("hello")
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "TextViewEdits", `
package test

fun update() {
    textView.setText("hello")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWrongViewCast(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "WrongViewCast", `
package test

fun setup() {
    val tv = findViewById<ImageView>(R.id.tv_title)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "WrongViewCast", `
package test

fun setup() {
    val tv = findViewById<TextView>(R.id.tv_title)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDeprecated(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Deprecated", `
package test

fun doWork() {
    val task = AsyncTask()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "Deprecated", `
package test

fun doWork() {
    val scope = CoroutineScope(Dispatchers.IO)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestRange(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Range", `
package test

fun apply() {
    view.setAlpha(300)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "Range", `
package test

fun apply() {
    view.setAlpha(128)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestResourceType(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceType", `
package test

fun load() {
    getString(R.drawable.icon)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceType", `
package test

fun load() {
    getString(R.string.hello)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestResourceAsColor(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceAsColor", `
package test

fun style() {
    view.setBackgroundColor(R.color.primary)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceAsColor", `
package test

fun style() {
    view.setBackgroundColor(ContextCompat.getColor(context, R.color.primary))
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSupportAnnotationUsage(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "SupportAnnotationUsage", `
package test

class MyActivity {
    @MainThread
    fun loadData() {
        val conn = HttpURLConnection()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "SupportAnnotationUsage", `
package test

class MyActivity {
    @MainThread
    fun updateUI() {
        textView.text = "hello"
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAccidentalOctal(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "AccidentalOctal", `
package test

val perms = 0755
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "AccidentalOctal", `
package test

val perms = 493
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAppCompatMethod(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

fun setup() {
    getActionBar()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

fun setup() {
    getSupportActionBar()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestInnerclassSeparator(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("com/example/Outer/Inner")
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("com.example.Outer$Inner")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestObjectAnimatorBinding(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test

fun animate() {
    ObjectAnimator.ofFloat(view, "fooBar", 0f, 1f)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ObjectAnimatorBinding", `
package test

fun animate() {
    ObjectAnimator.ofFloat(view, "alpha", 0f, 1f)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestPropertyEscape(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "PropertyEscape", `
package test

val s = "hello\q world"
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "PropertyEscape", `
package test

val s = "hello\n world"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestShortAlarm(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test

fun schedule() {
    alarmManager.setRepeating(AlarmManager.RTC, time, 5000, pendingIntent)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test

fun schedule() {
    alarmManager.setRepeating(AlarmManager.RTC, time, 60000, pendingIntent)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLocalSuppress(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("NotARealIssue")
fun doStuff() {}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("NewApi")
fun doStuff() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestPluralsCandidate(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "PluralsCandidate", `
package test

fun display(count: Int) {
    if (count == 1) {
        val msg = getString(R.string.item_single)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "PluralsCandidate", `
package test

fun display(count: Int) {
    val msg = resources.getQuantityString(R.plurals.items, count, count)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
