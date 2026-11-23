package rules_test

import "testing"

func TestCursorLoopDirectLambdaParameters(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"use", `db.query()?.use { cursor -> while (cursor.moveToNext()) { val value = cursor.getString(cursor.getColumnIndexOrThrow("name")) } }`, 1},
		{"let", `db.query()?.let { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
		{"also", `db.query()?.also { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
		{"forEach", `cursors.forEach { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
		{"labeled trailing lambda", `cursors.forEach outer@{ cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
		{"annotated trailing lambda", `cursors.forEach @Suppress("UNUSED_VARIABLE") { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
		{"labeled while loop", `cursor.use { row -> rows@ while (row.moveToNext()) row.getColumnIndex("name") }`, 1},
		{"annotated while loop", `cursor.use { row -> @Suppress("UNUSED_VARIABLE") while (row.moveToNext()) row.getColumnIndex("name") }`, 1},
		{"sole argument", `db.query()?.use({ cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") })`, 1},
		{"labeled sole argument conservative miss", `cursor.use(rows@{ row -> while (row.moveToNext()) row.getColumnIndex("name") })`, 0},
		{"annotated sole argument conservative miss", `cursor.use(@Suppress("UNUSED_VARIABLE") { row -> while (row.moveToNext()) row.getColumnIndex("name") })`, 0},
		{"implicit parameter conservative miss", `db.query()?.use { while (it.moveToNext()) it.getColumnIndex("name") }`, 0},
		{"implicit receiver member is not a parameter", `run { while (it.moveToNext()) it.getColumnIndex("name") }`, 0},
		{"member shadow", `cursor.use { outer -> while (outer.moveToNext()) outer.getColumnIndex("name") }`, 1},
		{"qualified SDK constant", `db.query()?.use { cursor -> while (cursor.moveToNext()) { val value = cursor.getString(cursor.getColumnIndexOrThrow(Settings.Global.AIRPLANE_MODE_ON)) } }`, 1},
		{"unqualified SDK constant FIR only", `db.query()?.use { cursor -> while (cursor.moveToNext()) cursor.getColumnIndex(AIRPLANE_MODE_ON) }`, 0},
		{"if between lambda and loop", `db.query()?.use { cursor -> if (ready) { while (cursor.moveToNext()) cursor.getColumnIndex("name") } }`, 0},
		{"lambda between parameter and loop", `db.query()?.use { cursor -> run { while (cursor.moveToNext()) cursor.getColumnIndex("name") } }`, 0},
		{"function between parameter and loop", `db.query()?.use { cursor -> fun nested() { while (cursor.moveToNext()) cursor.getColumnIndex("name") }; nested() }`, 0},
		{"object between parameter and loop", `db.query()?.use { cursor -> val reader = object { fun read() { while (cursor.moveToNext()) cursor.getColumnIndex("name") } } }`, 0},
		{"call in nested lambda", `db.query()?.use { cursor -> while (cursor.moveToNext()) run { cursor.getColumnIndex("name") } }`, 0},
		{"shadow in loop", `db.query()?.use { cursor -> while (cursor.moveToNext()) { val cursor = db.query(); cursor.getColumnIndex("name") } }`, 0},
		{"mutable shadow before loop", `db.query()?.use { cursor -> var cursor = db.query(); while (cursor.moveToNext()) { cursor.getColumnIndex("name"); cursor = db.query() } }`, 0},
		{"standalone lambda", `val callback = { cursor: Cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 0},
		{"nonsole argument", `consume({ cursor: Cursor -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }, other)`, 0},
		{"destructured parameter", `pairs.forEach { (name, cursor) -> while (cursor.moveToNext()) cursor.getColumnIndex("name") }`, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			code := `import android.provider.Settings; fun read(db: Db, cursors: List<Cursor>) { ` + test.body + ` }`
			if findings := runRuleByName(t, "CursorLoopWithColumnIndexInLoop", code); len(findings) != test.want {
				t.Fatalf("expected %d, got %v", test.want, findings)
			}
		})
	}
}
