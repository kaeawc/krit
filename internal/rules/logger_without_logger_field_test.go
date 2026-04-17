package rules_test

import "testing"

func TestLoggerWithoutLoggerField_Positive(t *testing.T) {
	findings := runRuleByName(t, "LoggerWithoutLoggerField", `
package test

object LoggerFactory {
    fun getLogger(target: Any): Logger = Logger()
}

class Logger {
    fun info(message: String) {}
}

class Handler {
    fun handle() {
        val log = LoggerFactory.getLogger(javaClass)
        log.info("handle")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestLoggerWithoutLoggerField_Negative(t *testing.T) {
	findings := runRuleByName(t, "LoggerWithoutLoggerField", `
package test

object LoggerFactory {
    fun getLogger(target: Any): Logger = Logger()
}

class Logger {
    fun info(message: String) {}
}

class Handler {
    private val log = LoggerFactory.getLogger(javaClass)

    fun handle() {
        log.info("handle")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}
