package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const tileCacheManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-feature android:name="android.hardware.type.watch" />
    <application android:label="Tiles">
        <service
            android:name=".MyTileService"
            android:exported="true"
            android:permission="com.google.android.wearable.permission.BIND_TILE_PROVIDER">
            <intent-filter>
                <action android:name="androidx.wear.tiles.action.BIND_TILE_PROVIDER" />
            </intent-filter>
            <meta-data android:name="androidx.wear.tiles.PREVIEW" android:resource="@drawable/tile_preview" />
        </service>
    </application>
</manifest>
`

func writeTilePNG(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}
}

// The tile preview checks read the drawables of every res directory of the manifest's module. A warm run must follow
// an edit to a preview that lives outside src/main/res, or next to a manifest in a flat module layout, exactly as a
// cold run does.
func TestTilePreviewWarmRunFollowsPreviewEdits(t *testing.T) {
	layouts := []struct {
		name, manifest, preview, roundDir string
	}{
		{"source set without main res", "app/src/main/AndroidManifest.xml", "app/src/wear/res/drawable/tile_preview.png",
			"app/src/debug/res/drawable-round"},
		{"flat module", "app/AndroidManifest.xml", "app/res/drawable/tile_preview.png", "app/res/drawable-round"},
	}
	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			for rel, content := range map[string]string{
				"app/build.gradle.kts": "plugins { id(\"com.android.application\") }\n",
				layout.manifest:        tileCacheManifest,
			} {
				path := filepath.Join(project, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			preview := filepath.Join(project, filepath.FromSlash(layout.preview))
			round := filepath.Join(project, filepath.FromSlash(layout.roundDir), "tile_preview.png")
			writeTilePNG(t, preview, 400)
			writeTilePNG(t, round, 400)
			count := func(rule string, args ...string) int {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--no-fir", "-f", "json"}, args...), project)...)
				var report struct {
					Findings []struct {
						Rule string `json:"rule"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				n := 0
				for _, f := range report.Findings {
					if f.Rule == rule {
						n++
					}
				}
				return n
			}
			if got := count("TilePreviewImageFormat"); got != 0 {
				t.Fatalf("cold run with a 400px preview reported %d TilePreviewImageFormat findings", got)
			}
			if got := count("SquareAndRoundTilePreviews"); got != 0 {
				t.Fatalf("cold run with round and square previews reported %d SquareAndRoundTilePreviews findings", got)
			}
			writeTilePNG(t, preview, 100)
			warm, cold := count("TilePreviewImageFormat"), count("TilePreviewImageFormat", "--no-cache")
			if cold == 0 || warm != cold {
				t.Fatalf("after shrinking the preview: warm run %d findings, cold run %d", warm, cold)
			}
			if err := os.RemoveAll(filepath.Dir(round)); err != nil {
				t.Fatal(err)
			}
			warm, cold = count("SquareAndRoundTilePreviews"), count("SquareAndRoundTilePreviews", "--no-cache")
			if cold == 0 || warm != cold {
				t.Fatalf("after deleting the round preview: warm run %d findings, cold run %d", warm, cold)
			}
		})
	}
}
