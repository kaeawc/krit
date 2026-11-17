package rules_test

import "testing"

// A class imported from another package under an Android simple name is a
// lookalike: the Go fallback must not report it (FIR resolves it and does not).
func TestAndroidSourceRulesSkipImportedLookalikes(t *testing.T) {
	cases := []struct {
		rule string
		code string
	}{
		{"ShowToast", `package test
import com.example.ui.Toast
fun f(ctx: Any) {
    Toast.makeText(ctx, "x", 2000)
}
`},
		{"ShortAlarm", `package test
import com.example.alarm.AlarmManager
fun f(am: AlarmManager, op: Any) {
    am.setRepeating(0, 100L, 200L, op)
}
`},
		{"SetTextI18n", `package test
import com.example.widget.TextView
class Screen(val tv: TextView) {
    fun f() {
        tv.setText("Hello there")
    }
}
`},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			if findings := runRuleWithIndexedResolver(t, tc.rule, tc.code); len(findings) != 0 {
				t.Fatalf("expected 0 findings, got %d", len(findings))
			}
		})
	}
}

// The platform classes stay reported.
func TestAndroidSourceRulesKeepPlatformImports(t *testing.T) {
	cases := []struct {
		rule string
		code string
		want int
	}{
		{"ShowToast", `package test
import android.widget.Toast
fun f(ctx: android.content.Context) {
    Toast.makeText(ctx, "x", 2000)
}
`, 2},
		{"ShortAlarm", `package test
import android.app.AlarmManager
fun f(am: AlarmManager, op: android.app.PendingIntent) {
    am.setRepeating(AlarmManager.RTC, 100L, 200L, op)
}
`, 2},
		{"SetTextI18n", `package test
import android.widget.TextView
class Screen(val tv: TextView) {
    fun f() {
        tv.setText("Hello there")
    }
}
`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			if findings := runRuleWithIndexedResolver(t, tc.rule, tc.code); len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}
