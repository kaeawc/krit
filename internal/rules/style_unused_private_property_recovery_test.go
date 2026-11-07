package rules_test

import "testing"

const unusedPrivatePropertySwallowedMembersSource = `
package test

class Holder(
  @param:TestParameter private val flag: Boolean
) : Base() {

  private val scheduler = TestCoroutineScheduler()

  @get:Rule val rule = Rule.fromScheduler(scheduler)

  private val fakeLabels = FakeLabels()
  private var relativeLabel: String? = "3d"
  private val labels =
    object : Labels by fakeLabels {
      override fun relative(
        moment: Instant?,
        short: Boolean,
      ): String? = relativeLabel
    }
  private val due = Instant.parse("2022-03-09T08:00:00Z")
  private val underTest: Reminder =
    ReminderImpl(
      labels = labels,
      retries = 1,
    )

  @Before
  fun setUp() {
    fakeLabels.add(due.epochSecond, "Mar 9")
    println(flag)
  }

  @Test
  fun ` + "`remind - none due - returns nothing`" + `() =
    runTest(scheduler) {
      underTest(items = emptyList()).test {
        assertThat(awaitItem()).isEqualTo(1)
      }
    }
}
`

func TestUnusedPrivateProperty_ReadAsNamedArgumentAfterErrorRecoveredInfixMembers(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateProperty", unusedPrivatePropertySwallowedMembersSource)
	if len(findings) != 0 {
		t.Fatalf("a read in a later property initializer must count, got %d", len(findings))
	}
}

func TestUnusedPrivateProperty_StillReportsUnusedWithErrorRecoveredInfixMembers(t *testing.T) {
	findings := runRuleByName(t, "UnusedPrivateProperty", `
package test

class Holder(
  @param:TestParameter private val flag: Boolean
) : Base() {

  private val fakeLabels = FakeLabels()
  private var relativeLabel: String? = "3d"
  private val unusedLabels =
    object : Labels by fakeLabels {
      override fun relative(
        moment: Instant?,
        short: Boolean,
      ): String? = relativeLabel
    }
  private val due = Instant.parse("2022-03-09T08:00:00Z")

  @Test
  fun `+"`remind - none due - returns nothing`"+`() =
    runTest(scheduler) {
      assertThat(due).isEqualTo(flag)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the unread property to be reported, got %d", len(findings))
	}
}
