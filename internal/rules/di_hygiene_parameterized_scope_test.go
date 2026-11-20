package rules_test

import (
	"fmt"
	"testing"
)

func TestScopeOnParameterizedClassFrameworkMessages(t *testing.T) {
	sharedMessage := "@%s on generic class 'Cache' shares one instance across all type arguments because the type parameter is erased at runtime."
	cdiMessage := "@%s on generic class 'Cache': CDI allows only @Dependent on a bean with type parameters, so this is a definition error."
	cases := []struct {
		annotation string
		name       string
		message    string
	}{
		{"org.koin.core.annotation.Singleton", "Singleton", sharedMessage},
		{"org.koin.core.annotation.Single", "Single", sharedMessage},
		{"javax.ejb.Singleton", "Singleton", sharedMessage},
		{"jakarta.ejb.Singleton", "Singleton", sharedMessage},
		{"javax.faces.bean.ApplicationScoped", "ApplicationScoped", sharedMessage},
		{"jakarta.faces.bean.ViewScoped", "ViewScoped", sharedMessage},
		{"jakarta.enterprise.context.ApplicationScoped", "ApplicationScoped", cdiMessage},
		{"javax.enterprise.context.SessionScoped", "SessionScoped", cdiMessage},
		{"jakarta.enterprise.context.ConversationScoped", "ConversationScoped", cdiMessage},
		{"javax.faces.view.ViewScoped", "ViewScoped", cdiMessage},
		{"jakarta.faces.view.ViewScoped", "ViewScoped", cdiMessage},
		{"javax.faces.flow.FlowScoped", "FlowScoped", cdiMessage},
		{"jakarta.faces.flow.FlowScoped", "FlowScoped", cdiMessage},
		{"javax.transaction.TransactionScoped", "TransactionScoped", cdiMessage},
		{"jakarta.transaction.TransactionScoped", "TransactionScoped", cdiMessage},
	}
	rule := buildRuleIndex()["ScopeOnParameterizedClass"]
	for _, tc := range cases {
		t.Run(tc.annotation, func(t *testing.T) {
			for _, code := range []string{
				fmt.Sprintf("import %s\n@%s class Cache<T>", tc.annotation, tc.name),
				fmt.Sprintf("import %s as BeanScope\n@BeanScope class Cache<T>", tc.annotation),
				fmt.Sprintf("package demo\n@%s class Cache<T>", tc.annotation),
				fmt.Sprintf("import %s\n@[%s] class Cache<T>", tc.annotation, tc.name),
				fmt.Sprintf("import %s\n@[A %s] class Cache<T>", tc.annotation, tc.name),
				fmt.Sprintf("import %s as BeanScope\n@[A BeanScope] class Cache<T>", tc.annotation),
				fmt.Sprintf("package demo\n@[A %s] class Cache<T>", tc.annotation),
				fmt.Sprintf("import %s\n@[A(\"Singleton\") %s()] class Cache<T>", tc.annotation, tc.name),
			} {
				findings := runRule(t, rule, parseInline(t, code))
				if len(findings) != 1 {
					t.Fatalf("findings = %v, want one for %s", findings, code)
				}
				if want := fmt.Sprintf(tc.message, tc.name); findings[0].Message != want {
					t.Fatalf("message = %q, want %q", findings[0].Message, want)
				}
				if findings[0].Line != 2 {
					t.Fatalf("line = %d, want 2", findings[0].Line)
				}
			}
		})
	}
}

func TestScopeOnParameterizedClassUnresolvedAnnotations(t *testing.T) {
	rule := buildRuleIndex()["ScopeOnParameterizedClass"]
	for _, code := range []string{
		"@[Singleton] class Cache<T>",
		"@[A Single] class Cache<T>",
		"import org.koin.core.annotation.*\n@[A Singleton] class Cache<T>",
		"import org.koin.core.annotation.*\n@Single class Cache<T>",
		"import example.Single\n@[A Single] class Cache<T>",
		"import jakarta.enterprise.context.ViewScoped\n@ViewScoped class Cache<T>",
		"import javax.enterprise.context.ViewScoped\n@ViewScoped class Cache<T>",
		"import org.koin.core.annotation.Singleton\n@[A(\"Singleton\")] class Cache<T>",
		"import org.koin.core.annotation.Singleton\n@[A(Singleton::class)] class Cache<T>",
	} {
		t.Run(code, func(t *testing.T) {
			if findings := runRule(t, rule, parseInline(t, code)); len(findings) != 0 {
				t.Fatalf("findings = %v, want none", findings)
			}
		})
	}
}
