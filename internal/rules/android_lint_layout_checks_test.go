package rules_test

import (
	"reflect"
	"testing"
)

const layoutChecksSample = `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content">
    <Button android:layout_width="wrap_content" android:layout_height="wrap_content"/>
    <requestFocus/>
  </TextView>
  <com.example.CustomView android:layout_width="wrap_content" android:layout_height="wrap_content">
    <Button android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  </com.example.CustomView>
  <androidx.constraintlayout.widget.ConstraintLayout android:layout_width="wrap_content" android:layout_height="wrap_content">
    <Button android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  </androidx.constraintlayout.widget.ConstraintLayout>
  <ImageView android:layout_width="wrap_content" android:layout_height="wrap_content">
    <include layout="@layout/b"/>
    <tag android:id="@+id/x"/>
  </ImageView>
  <androidx.recyclerview.widget.RecyclerView android:layout_width="wrap_content" android:layout_height="wrap_content">
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  </androidx.recyclerview.widget.RecyclerView>
  <android.support.design.bottomappbar.BottomAppBar android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="@+id/foo"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="6"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="abc"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="@integer/ime"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="@string/ime"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="@android:integer/x"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="?attr/foo"/>
  <EditText android:layout_width="wrap_content" android:layout_height="wrap_content" android:imeActionId="0x10"/>
</LinearLayout>`

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestLayoutLintChecksLintParity(t *testing.T) {
	files := map[string]string{
		"layout/a.xml": layoutChecksSample,
		"layout/c.xml": `<androidx.coordinatorlayout.widget.CoordinatorLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <com.google.android.material.bottomappbar.BottomAppBar android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</androidx.coordinatorlayout.widget.CoordinatorLayout>`,
		"menu/m.xml": `<menu xmlns:android="http://schemas.android.com/apk/res/android">
  <item android:id="@+id/a"/>
  <item android:id="@+id/b" android:title="B"/>
  <item android:id="@+id/c" android:visible="false"/>
  <item android:id="@+id/d" android:title="TODO"/>
  <item android:id="@+id/e" android:title=""/>
  <group android:id="@+id/g"><item android:id="@+id/f"/></group>
</menu>`,
	}
	want := map[string][]string{
		"ChildInNonViewGroupResource": {"layout/a.xml:3:6", "layout/a.xml:13:6"},
		"BottomAppBar":                {"layout/a.xml:19:4"},
		"InvalidImeActionId": {
			"layout/a.xml:20:86", "layout/a.xml:22:86", "layout/a.xml:24:86", "layout/a.xml:26:86", "layout/a.xml:27:86",
		},
		"MenuTitle": {"menu/m.xml:2:4", "menu/m.xml:7:31"},
	}
	for rule, positions := range want {
		t.Run(rule, func(t *testing.T) {
			if got := resourcePositions(t, rule, files); !reflect.DeepEqual(got, positions) {
				t.Fatalf("positions:\n got  %v\n want %v", got, positions)
			}
		})
	}
}
