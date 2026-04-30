package rules

import "testing"

func TestRoomQueryMissingWhereSQL(t *testing.T) {
	cases := []struct {
		sql     string
		missing bool
		kw      string
	}{
		{"DELETE FROM users", true, "DELETE"},
		{"  delete from users", true, "DELETE"},
		{"UPDATE users SET name = 'x'", true, "UPDATE"},
		{"DELETE FROM users WHERE id = 1", false, ""},
		{"UPDATE users SET name = 'x' where id = 1", false, ""},
		{"SELECT * FROM users", false, ""},
		{"INSERT INTO users(id) VALUES(1)", false, ""},
		{"\n        DELETE FROM users\n    ", true, "DELETE"},
	}
	for _, c := range cases {
		kw, missing := roomQueryMissingWhereSQL(c.sql)
		if missing != c.missing || kw != c.kw {
			t.Errorf("sql=%q: got (%q,%v), want (%q,%v)", c.sql, kw, missing, c.kw, c.missing)
		}
	}
}

func TestRoomQueryAnnotationSQL(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{`@Query("DELETE FROM users")`, "DELETE FROM users"},
		{`@Query(value = "DELETE FROM users")`, "DELETE FROM users"},
		{`@Query("""DELETE FROM users""")`, "DELETE FROM users"},
		{`@Query("DELETE FROM users WHERE id = :id")`, "DELETE FROM users WHERE id = :id"},
	}
	for _, c := range cases {
		got := roomQueryAnnotationSQL(c.text)
		if got != c.want {
			t.Errorf("text=%q: got %q, want %q", c.text, got, c.want)
		}
	}
}

func TestRoomQueryFunctionNameAllowsFullTable(t *testing.T) {
	allow := []string{"deleteAll", "deleteAllUsers", "clearAll", "clearAllSessions", "DELETEALL"}
	deny := []string{"delete", "purge", "wipe", "removeUser", ""}
	for _, n := range allow {
		if !roomQueryFunctionNameAllowsFullTable(n) {
			t.Errorf("expected %q to allow full-table", n)
		}
	}
	for _, n := range deny {
		if roomQueryFunctionNameAllowsFullTable(n) {
			t.Errorf("expected %q to NOT allow full-table", n)
		}
	}
}
