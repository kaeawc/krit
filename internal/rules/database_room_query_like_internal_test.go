package rules

import "testing"

func TestRoomQueryLikeBindMissingEscape(t *testing.T) {
	cases := []struct {
		sql     string
		missing bool
		bind    string
	}{
		{"SELECT * FROM users WHERE name LIKE :q", true, "q"},
		{"SELECT * FROM users WHERE name like :prefix", true, "prefix"},
		{"SELECT * FROM users WHERE name LIKE '%' || :q || '%' ESCAPE '\\'", false, ""},
		{"SELECT * FROM users WHERE name LIKE :q || '%'", false, ""},
		{"SELECT * FROM users WHERE name = :q", false, ""},
		{"SELECT * FROM users WHERE name LIKE :q ESCAPE '\\'", false, ""},
		{"SELECT * FROM users WHERE id = :id", false, ""},
	}
	for _, c := range cases {
		bind, missing := roomQueryLikeBindMissingEscape(c.sql)
		if missing != c.missing || bind != c.bind {
			t.Errorf("sql=%q: got (%q,%v), want (%q,%v)", c.sql, bind, missing, c.bind, c.missing)
		}
	}
}
