package rules_test

import "testing"

func TestTestWithoutAssertion_FrameworkAssertionForms(t *testing.T) {
	negatives := map[string]string{
		"turbine test with awaitItem": `
    @Test
    fun emits() = runTest {
        flow.test {
            awaitItem()
            awaitComplete()
        }
    }`,
		"turbine expectNoEvents": `
    @Test
    fun quiet() = runTest {
        flow.test { expectNoEvents() }
    }`,
		"compose assertIsDisplayed chain": `
    @Test
    fun shows() {
        composeTestRule.onNodeWithTag("a").assertIsDisplayed()
    }`,
		"compose waitUntil": `
    @Test
    fun waits() {
        composeTestRule.waitUntil { true }
    }`,
		"compose waitUntil with timeout": `
    @Test
    fun waits() {
        composeTestRule.waitUntil(timeoutMillis = 1000) { state.ready }
    }`,
		"compose runOnIdle with assert": `
    @Test
    fun idle() {
        composeTestRule.runOnIdle { assertEquals(1, counter) }
    }`,
		"mockk verify": `
    @Test
    fun calls() {
        sut.run()
        verify { dep.call() }
    }`,
		"mockk coVerify": `
    @Test
    fun calls() = runTest {
        sut.run()
        coVerify(exactly = 1) { dep.call() }
    }`,
		"mockito verify": `
    @Test
    fun calls() {
        sut.run()
        verify(dep).call()
    }`,
		"truth assertThat": `
    @Test
    fun checks() {
        assertThat(sut.value).isEqualTo(1)
    }`,
		"kotlin test assertFailsWith": `
    @Test
    fun fails() {
        assertFailsWith<IllegalStateException> { sut.run() }
    }`,
		"junit expected": `
    @Test(expected = IllegalStateException::class)
    fun fails() {
        sut.run()
    }`,
		"assertThrows": `
    @Test
    fun fails() {
        assertThrows(IllegalStateException::class.java) { sut.run() }
    }`,
		"Assert.fail": `
    @Test
    fun fails() {
        try { sut.run() } catch (e: Exception) { return }
        Assert.fail("expected an exception")
    }`,
		"runTest with assertEquals": `
    @Test
    fun checks() = runTest {
        sut.run()
        assertEquals(1, sut.value)
    }`,
		"rx assertNoErrors": `
    @Test
    fun emits() {
        val observer = source.test()
        observer.assertNoErrors()
    }`,
		"rx test observer chain": `
    @Test
    fun emits() {
        source.test().assertValue(1).assertComplete()
    }`,
		"rx awaitTerminalEvent": `
    @Test
    fun emits() {
        source.test().awaitDone(1, TimeUnit.SECONDS)
    }`,
		"robolectric shadow assertion": `
    @Test
    fun starts() {
        sut.start()
        assertThat(shadowOf(activity).nextStartedActivity).isNotNull()
    }`,
	}
	for name, body := range negatives {
		t.Run("negative/"+name, func(t *testing.T) {
			findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class FrameworkFormsTest {`+body+`
}
`)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
			}
		})
	}

	positives := map[string]string{
		"mock call without verify": `
    @Test
    fun calls() {
        every { dep.call() } returns 1
        sut.run()
    }`,
		"mockito when without verify": `
    @Test
    fun calls() {
        whenever(dep.call()).thenReturn(1)
        sut.run()
    }`,
		"robolectric setup only": `
    @Test
    fun starts() {
        val controller = Robolectric.buildActivity(MainActivity::class.java)
        controller.create().start()
    }`,
		"compose runOnIdle without assert": `
    @Test
    fun idle() {
        composeTestRule.runOnIdle { counter++ }
    }`,
	}
	for name, body := range positives {
		t.Run("positive/"+name, func(t *testing.T) {
			findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class FrameworkFormsTest {`+body+`
}
`)
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
			}
		})
	}
}
