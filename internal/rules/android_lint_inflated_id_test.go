package rules_test

import "testing"

var inflatedIDResources = map[string]string{
	"layout/main.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:id="@+id/title" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  <TextView android:id="@id/ref" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</LinearLayout>`,
	"layout-land/main.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:id="@+id/title" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  <TextView android:id="@+id/landOnly" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</LinearLayout>`,
	"layout/withinc.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <include layout="@layout/main"/>
</LinearLayout>`,
	"layout/other.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:id="@+id/other" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</LinearLayout>`,
}

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestMissingInflatedIdJavaLintParity(t *testing.T) {
	got := sourcePositionsWithResources(t, "MissingInflatedId", "A.java", `package com.example;
import android.app.Activity;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
public class A extends Activity {
  @Override protected void onCreate(Bundle b) {
    super.onCreate(b);
    setContentView(R.layout.main);
    View a = findViewById(R.id.title);
    View c = findViewById(R.id.missing);
    View d = findViewById(R.id.landOnly);
    View e = findViewById(R.id.ref);
    View f = findViewById(android.R.id.content);
    View g = requireViewById(R.id.missing2);
    if (b != null) { View h = findViewById(R.id.missing3); }
  }
  void other(LayoutInflater inflater, ViewGroup parent) {
    View v = inflater.inflate(R.layout.other, parent, false);
    View x = v.findViewById(R.id.other);
    View y = v.findViewById(R.id.nothere);
    View z = parent.findViewById(R.id.nothere2);
  }
  void inc() {
    setContentView(R.layout.withinc);
    View q = findViewById(R.id.anything);
  }
  void late() {
    View q = findViewById(R.id.late1);
    setContentView(R.layout.other);
  }
}
`, inflatedIDResources)
	if want := "12:27,16:30,22:29,30:27"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestMissingInflatedIdKotlinLintParity(t *testing.T) {
	got := sourcePositionsWithResources(t, "MissingInflatedId", "B.kt", `package com.example
import android.app.Activity
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
class B : Activity() {
  override fun onCreate(b: Bundle?) {
    super.onCreate(b)
    setContentView(R.layout.main)
    val a = findViewById<View>(R.id.title)
    val c = findViewById<View>(R.id.ktmissing)
    this.setContentView(R.layout.other)
  }
  fun f(inflater: LayoutInflater, parent: ViewGroup) {
    val v = inflater.inflate(R.layout.other, parent, false)
    v.findViewById<View>(R.id.ktmissing2)
  }
  fun g() {
    setContentView(R.layout.other)
    findViewById<View>(R.id.ktmissing5)
    run { findViewById<View>(R.id.ktmissing6) }
  }
  fun h() = findViewById<View>(R.id.ktmissing7)
}
`, inflatedIDResources)
	if want := "12:32,21:24"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
