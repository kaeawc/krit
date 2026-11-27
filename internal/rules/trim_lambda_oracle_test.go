package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
)

// krit-fir types a nullable receiver as `kotlin.String?`. TrimLambda's String
// check once compared that FQN as written and dropped `maybe?.trim { }`
// whenever the oracle ran (#1869).
func TestTrimLambda_NullableReceiverTypedByOracle(t *testing.T) {
	const code = "fun clean(maybe: String?): String? = maybe?.trim { it.isWhitespace() }\n"
	facts := func(file *scanner.File) map[string]*oracle.ExpressionType {
		out := map[string]*oracle.ExpressionType{}
		for i := range file.FlatTree.Types {
			node := uint32(i)
			if file.FlatType(node) == "simple_identifier" && file.FlatNodeText(node) == "maybe" {
				out[positionKey(file, node)] = &oracle.ExpressionType{Type: "kotlin.String?", Nullable: true,
					StartByte: int(file.FlatStartByte(node)), EndByte: int(file.FlatEndByte(node))}
			}
		}
		return out
	}
	if got := runWithOracleFacts(t, "TrimLambda", code, facts); len(got) != 1 {
		t.Fatalf("TrimLambda with the oracle typing maybe as kotlin.String?: %d findings, want 1: %v", len(got), got)
	}
}
