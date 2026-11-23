package rules

import "testing"

func TestDatabaseHasRoomSourceEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want bool
	}{
		{"annotatedWithImport", "import androidx.room.Dao\n\n@Dao\ninterface UserDao\n", true},
		{"annotatedWithoutRoomReference", "import com.example.Dao\n\n@Dao\ninterface UserDao\n", false},
		{"roomImportWithoutAnnotation", "import androidx.room.Dao\n\ninterface UserDao\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := parseCoroutinesInline(t, tc.code)
			var decl uint32
			file.FlatWalkAllNodes(0, func(idx uint32) {
				if decl == 0 && file.FlatType(idx) == "class_declaration" {
					decl = idx
				}
			})
			if decl == 0 {
				t.Fatal("no class_declaration")
			}
			if got := databaseHasRoomSourceEvidence(file, decl, []string{"Dao"}); got != tc.want {
				t.Errorf("databaseHasRoomSourceEvidence = %v, want %v", got, tc.want)
			}
		})
	}
}
