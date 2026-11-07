package rules_test

import (
	"strings"
	"testing"
)

// The fixture and the expected findings were produced by running Android Lint 32.4.1 on the same files.
func TestTextFieldsLintParity(t *testing.T) {
	files := map[string]string{
		"values/styles.xml": `<resources>
  <style name="WithType"><item name="android:inputType">text</item></style>
  <style name="NoType"><item name="android:textSize">12sp</item></style>
  <style name="Child" parent="@style/WithType"/>
  <style name="WithType.Dot"/>
</resources>
`,
		"layout/tf.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <EditText android:id="@+id/a1" android:layout_width="wrap_content" android:layout_height="wrap_content" android:hint="h"/>
  <EditText android:id="@+id/a2" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/WithType"/>
  <EditText android:id="@+id/a3" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/NoType"/>
  <EditText android:id="@+id/a4" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputMethod="x"/>
  <EditText android:id="@+id/a5" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="TODO"/>
  <EditText android:id="@+id/a6" android:layout_width="wrap_content" android:layout_height="wrap_content" android:digits="123"/>
  <EditText android:id="@+id/a7" android:layout_width="wrap_content" android:layout_height="wrap_content" android:password="true"/>
  <EditText android:id="@+id/phone" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/WithType"/>
  <EditText android:id="@+id/b2" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/Missing"/>
  <EditText android:id="@+id/b3" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/Child"/>
  <EditText android:id="@+id/b4" android:layout_width="wrap_content" android:layout_height="wrap_content" style="@style/WithType.Dot"/>
  <EditText android:id="@+id/passwordField" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text" android:password="true"/>
  <EditText android:id="@+id/phone2" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text" android:phoneNumber="true"/>
  <EditText android:id="@+id/someDate" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/update" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/PIN" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/pinCode" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/zip_pin" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/numberOfItems" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/screenWidth" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/passwordConfirm" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/homeUrl" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/editTextEmail" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <EditText android:id="@+id/editText3" android:layout_width="wrap_content" android:layout_height="wrap_content" android:inputType="text"/>
  <com.example.MyEdit android:id="@+id/custom" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
  <androidx.appcompat.widget.AppCompatEditText android:id="@+id/compat" android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</LinearLayout>
`,
	}
	got := resourceFindings(t, "TextFieldsResource", files)
	const noType = "This text field does not specify an `inputType`"
	want := []string{
		"layout/tf.xml:02 " + noType,
		"layout/tf.xml:04 " + noType,
		"layout/tf.xml:06 " + noType,
		"layout/tf.xml:07 " + noType,
		"layout/tf.xml:08 " + noType,
		"layout/tf.xml:09 The view name (`@+id/phone`) suggests this is a phone number, but it does not include '`phone`' in the `inputType`",
		"layout/tf.xml:15 The view name (`@+id/someDate`) suggests this is a date, but it does not include '`date`' or '`datetime`' in the `inputType`",
		"layout/tf.xml:17 The view name (`@+id/PIN`) suggests this is a password, but it does not include '`numberPassword`' in the `inputType`",
		"layout/tf.xml:19 The view name (`@+id/zip_pin`) suggests this is a password, but it does not include '`numberPassword`' in the `inputType`",
		"layout/tf.xml:20 The view name (`@+id/numberOfItems`) suggests this is a number, but it does not include a numeric `inputType` (such as '`numberSigned`')",
		"layout/tf.xml:21 The view name (`@+id/screenWidth`) suggests this is a number, but it does not include a numeric `inputType` (such as '`numberSigned`')",
		"layout/tf.xml:22 The view name (`@+id/passwordConfirm`) suggests this is a password, but it does not include '`textPassword`' in the `inputType`",
		"layout/tf.xml:23 The view name (`@+id/homeUrl`) suggests this is a URI, but it does not include '`textUri`' in the `inputType`",
		"layout/tf.xml:24 The view name (`@+id/editTextEmail`) suggests this is an e-mail address, but it does not include '`textEmail`' in the `inputType`",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
