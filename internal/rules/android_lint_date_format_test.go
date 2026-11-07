package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same sources.
func TestWeekBasedYearJavaLintParity(t *testing.T) {
	got := findingPositions(runRuleByNameOnJava(t, "WeekBasedYear", `package p;
import java.time.format.DateTimeFormatter;
public class A {
  static final String P = "YYYY-MM-dd";
  void f(String dyn) {
    DateTimeFormatter.ofPattern("YYYY-MM-dd");
    DateTimeFormatter.ofPattern("yyyy-MM-dd");
    DateTimeFormatter.ofPattern("YYYY");
    DateTimeFormatter.ofPattern("YYYY-MM-dd'Y'");
    DateTimeFormatter.ofPattern("'Y'-MM-dd");
    DateTimeFormatter.ofPattern("YY-MM-dd");
    DateTimeFormatter.ofPattern("MM-dd YYYY yyyy");
    DateTimeFormatter.ofPattern(P);
    DateTimeFormatter.ofPattern("YYYY-" + dyn + "-dd");
    DateTimeFormatter.ofPattern(dyn);
    DateTimeFormatter.ofPattern("dd/MM/YYYY", java.util.Locale.US);
    DateTimeFormatter.ofPattern("uuuu-MM-dd YYYY");
    DateTimeFormatter.ofPattern("Y");
    DateTimeFormatter.ofPattern("YYYY " + "MM");
    new java.text.SimpleDateFormat("YYYY-MM-dd", java.util.Locale.US);
  }
}
`))
	want := "6:33,9:33,11:33,13:33,14:33,16:39,19:33,20:36"
	if got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestWeekBasedYearKotlinLintParity(t *testing.T) {
	got := findingPositions(runRuleByName(t, "WeekBasedYear", `package p
import java.time.format.DateTimeFormatter
class B {
  val P = "YYYY-MM-dd"
  fun f(dyn: String) {
    DateTimeFormatter.ofPattern("YYYY-MM-dd")
    DateTimeFormatter.ofPattern(P)
    DateTimeFormatter.ofPattern("YYYY-$dyn-dd")
    DateTimeFormatter.ofPattern("""YYYY-MM""")
    DateTimeFormatter.ofPattern("dd-YY")
    DateTimeFormatter.ofPattern("yyyy-MM")
  }
}
`))
	want := "6:34,7:33,8:33,9:36,10:37"
	if got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
