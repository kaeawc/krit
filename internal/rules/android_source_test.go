package rules_test

import (
	"testing"
)

func TestFragmentConstructor(t *testing.T) {
	t.Run("parameterized primary constructor without default triggers", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int) : Fragment() {
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no explicit constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("all default parameters does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int = 0, val name: String = "") : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("secondary no-arg constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment : Fragment {
    constructor() : super()
    constructor(id: Int) : super()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-fragment class does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
class MyHelper(val id: Int) {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("parameterized primary plus secondary no-arg constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int) : Fragment() {
    constructor() : this(0)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestServiceCast(t *testing.T) {
	t.Run("flags wrong service cast", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("correct service cast does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as AlarmManager
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no getSystemService does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getManager() as PowerManager
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestShowToast(t *testing.T) {
	t.Run("flags Toast.makeText without show", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Toast.makeText with chained show does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no Toast usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSparseArray(t *testing.T) {
	t.Run("flags HashMap with Int key", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
class Foo {
    val map = HashMap<Int, String>()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags HashMap with Integer key", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
class Foo {
    val map = HashMap<Integer, String>()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags HashMap with Long key suggests LongSparseArray", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
class Foo {
    val map = HashMap<Long, String>()
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("HashMap with String key does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
class Foo {
    val map = HashMap<String, String>()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestUseValueOf(t *testing.T) {
	t.Run("flags Integer constructor call", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val num = Integer(42)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags Boolean constructor call", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val flag = Boolean(true)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("valueOf usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val num = Integer.valueOf(42)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("flags new Integer in Java source", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Integer num = new Integer(42);
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Java valueOf does not trigger", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Integer num = Integer.valueOf(42);
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("Java anonymous subclass does not trigger", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Object o = new Integer(42) { };
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLogTag(t *testing.T) {
	t.Run("flags tag over 23 characters", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("ThisIsAVeryLongTagNameThatExceedsTwentyThreeChars", "message")
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("tag within limit does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("MyTag", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("tag exactly 23 chars does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("12345678901234567890123", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLogTagMismatch(t *testing.T) {
	t.Run("flags TAG value not matching class name", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }
    fun doStuff() {
        Log.d(TAG, "message")
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("TAG matching class name does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "MyActivity"
    }
    fun doStuff() {
        Log.d(TAG, "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no TAG constant does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    fun doStuff() {
        Log.d("MyActivity", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestNonInternationalizedSms(t *testing.T) {
	t.Run("flags SmsManager.sendTextMessage with domestic literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send() {
        SmsManager.sendTextMessage("5551234567", null, "Hello", null, null)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags smsManager.sendMultipartTextMessage with domestic literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send(smsManager: SmsManager) {
        smsManager.sendMultipartTextMessage("5551234567", null, parts, null, null)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("does not trigger for E.164 literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send() {
        SmsManager.sendTextMessage("+15551234567", null, "Hello", null, null)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("does not trigger for dynamic destination", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send(smsManager: SmsManager, dest: String) {
        smsManager.sendTextMessage(dest, null, "Hello", null, null)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no SMS usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLayoutInflation(t *testing.T) {
	t.Run("flags inflate with null parent", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `
package test
class Foo {
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.main, null)
    }
}`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent")
		}
	})

	t.Run("ignores inflate with proper parent", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `
package test
class Foo {
    fun onCreateView(inflater: LayoutInflater, container: ViewGroup?): View {
        return inflater.inflate(R.layout.main, container, false)
    }
}`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for inflate with parent, got %d", len(findings))
		}
	})
}

func TestGetSignatures(t *testing.T) {
	t.Run("flags GET_SIGNATURES package info flag usage", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `
package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }
}`)
		if len(findings) == 0 {
			t.Fatal("expected finding for GET_SIGNATURES")
		}
	})

	t.Run("ignores GET_SIGNING_CERTIFICATES", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `
package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for GET_SIGNING_CERTIFICATES, got %d", len(findings))
		}
	})
}
