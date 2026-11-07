package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// A krit-fir jar swap must rotate every rule hash that keys persisted findings.
func TestProjectRuleHashForArgs_TracksOracleJarIdentity(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	t.Setenv("KRIT_FIR_JAR", jar)
	if err := os.WriteFile(jar, []byte("jar one"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := ProjectArgs{}
	before := projectRuleHashForArgs(args, false)
	if again := projectRuleHashForArgs(args, false); again != before {
		t.Fatalf("hash is not stable for an unchanged jar: %s != %s", again, before)
	}
	if err := os.WriteFile(jar, []byte("jar two, new diagnostic payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if after := projectRuleHashForArgs(args, false); after == before {
		t.Fatal("rule hash survived a krit-fir jar content change")
	}
}

func TestIndexRuleHash_TracksOracleJarIdentity(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	t.Setenv("KRIT_FIR_JAR", jar)
	hashFor := func(content string) string {
		if err := os.WriteFile(jar, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		var res IndexResult
		IndexPhase{}.computeRuleHash(IndexInput{}, &res)
		return res.RuleHash
	}
	if hashFor("alpha jar") == hashFor("beta jar bytes") {
		t.Fatal("IndexPhase rule hash ignores the krit-fir jar")
	}
}
