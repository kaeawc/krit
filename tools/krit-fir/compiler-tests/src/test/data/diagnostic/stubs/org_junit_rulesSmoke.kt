package stubs

fun junitRulesSmoke(thrown: org.junit.rules.ExpectedException, collector: org.junit.rules.ErrorCollector) {
    thrown.expect(IllegalArgumentException::class.java)
    collector.checkThat(1, Any())
    org.junit.rules.ExpectedException.none()
}
