package rules_test

import "testing"

func TestClickableViewAccessibilityAbstractClasses(t *testing.T) {
	t.Run("Kotlin", func(t *testing.T) {
		findings := runRuleByName(t, "ClickableViewAccessibility", `
import android.content.Context
import android.view.MotionEvent
import android.view.View
abstract class AbstractTouch(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
}
sealed class SealedTouch(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
}
sealed class SealedClick(context: Context) : View(context) {
    override fun onTouchEvent(event: MotionEvent): Boolean = true
    override fun performClick(): Boolean = true
}
abstract class AbstractClick(context: Context) : View(context) {
    abstract override fun onTouchEvent(event: MotionEvent): Boolean
    override fun performClick(): Boolean = true
    fun attach(view: View) {
        view.setOnTouchListener { _, _ -> true }
    }
}
`)
		if len(findings) != 1 || findings[0].Message != "`onTouch` lambda should call `View#performClick` when a click is detected" {
			t.Fatalf("expected only the listener finding, got %v", findings)
		}
	})
	t.Run("Java", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "ClickableViewAccessibility", `
import android.content.Context;
import android.view.MotionEvent;
import android.view.View;
abstract class AbstractTouch extends View {
    AbstractTouch(Context context) { super(context); }
    @Override public boolean onTouchEvent(MotionEvent event) { return true; }
}
abstract class AbstractClick extends View {
    AbstractClick(Context context) { super(context); }
    @Override public abstract boolean onTouchEvent(MotionEvent event);
    @Override public boolean performClick() { return true; }
    void attach(View view) {
        view.setOnTouchListener((target, event) -> true);
    }
}
`)
		if len(findings) != 1 || findings[0].Message != "`onTouch` lambda should call `View#performClick` when a click is detected" {
			t.Fatalf("expected only the listener finding, got %v", findings)
		}
	})
}

func TestClickableViewAccessibilityAnonymousObjects(t *testing.T) {
	findings := runRuleByName(t, "ClickableViewAccessibility", `
import android.content.Context
import android.view.MotionEvent
import android.view.View
interface TouchContract {
    fun onTouchEvent(event: MotionEvent): Boolean
    fun performClick(): Boolean
}
class TouchHost(context: Context) : View(context) {
    val callback = object : TouchContract {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}
class AnonymousViewHost(context: Context) : View(context) {
    val child = object : View(context) {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected anonymous objects to stay silent, got %v", findings)
	}
}

func TestClickableViewAccessibilityCompanionObjects(t *testing.T) {
	findings := runRuleByName(t, "ClickableViewAccessibility", `
import android.content.Context
import android.view.MotionEvent
import android.view.View
interface TouchContract {
    fun onTouchEvent(event: MotionEvent): Boolean
    fun performClick(): Boolean
}
class CompanionHost(context: Context) : View(context) {
    companion object : TouchContract {
        override fun onTouchEvent(event: MotionEvent): Boolean = true
        override fun performClick(): Boolean = true
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected companion objects to stay silent, got %v", findings)
	}
}

func TestClickableViewAccessibilityJavaNestedOwners(t *testing.T) {
	for name, member := range map[string]string{
		"anonymous listener": `TouchContract listener = new TouchContract() {
            public boolean onTouchEvent(MotionEvent event) { return true; }
            public boolean performClick() { return true; }
        };`,
		"nested interface": `interface Handler {
            boolean onTouchEvent(MotionEvent event);
            default boolean performClick() { return true; }
        }`,
		"nested enum": `enum Handler {
            INSTANCE;
            public boolean onTouchEvent(MotionEvent event) { return true; }
            public boolean performClick() { return true; }
        }`,
		"nested record": `record Handler(int id) {
            boolean onTouchEvent(MotionEvent event) { return true; }
            boolean performClick() { return true; }
        }`,
		"anonymous View": `View child = new View(null) {
            @Override public boolean onTouchEvent(MotionEvent event) { return true; }
            @Override public boolean performClick() { return true; }
        };`,
	} {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByNameOnJava(t, "ClickableViewAccessibility", `
import android.content.Context;
import android.view.MotionEvent;
import android.view.View;
interface TouchContract {
    boolean onTouchEvent(MotionEvent event);
    boolean performClick();
}
class Host extends View {
    Host(Context context) { super(context); }
`+member+`}
`)
			if len(findings) != 0 {
				t.Fatalf("expected nested owner to stay silent, got %v", findings)
			}
		})
	}
}

func TestClickableViewAccessibilityJavaSealedClassReports(t *testing.T) {
	findings := runRuleByNameOnJava(t, "ClickableViewAccessibility", `
import android.content.Context;
import android.view.MotionEvent;
import android.view.View;
sealed class SealedTouch extends View permits Child {
    SealedTouch(Context context) { super(context); }
    @Override public boolean onTouchEvent(MotionEvent event) { return true; }
}
final class Child extends SealedTouch {
    Child(Context context) { super(context); }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected Java sealed View override to report, got %v", findings)
	}
}
