package android

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const androidResNamespace = "http://schemas.android.com/apk/res/android"

// iconManifestInfo is what the icon checks read from the manifest that sits next to a res directory.
type iconManifestInfo struct {
	launcher map[string]bool
	round    map[string]bool
	minSdk   int
}

// readIconManifest extracts the icons that application, activity and alias elements declare, plus the
// minSdkVersion. Drawable references are stored by bare name and mipmap references as "@mipmap/name".
func readIconManifest(resDir string) iconManifestInfo {
	info := iconManifestInfo{launcher: map[string]bool{}, round: map[string]bool{}}
	f, err := os.Open(filepath.Join(filepath.Dir(resDir), "AndroidManifest.xml"))
	if err != nil {
		return info
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			return info
		}
		if start, ok := tok.(xml.StartElement); ok {
			info.visit(start)
		}
	}
}

func (m *iconManifestInfo) visit(start xml.StartElement) {
	tag := start.Name.Local
	if tag == "uses-sdk" {
		for _, a := range start.Attr {
			if a.Name.Space == androidResNamespace && a.Name.Local == "minSdkVersion" {
				m.minSdk, _ = strconv.Atoi(strings.TrimSpace(a.Value))
			}
		}
		return
	}
	switch tag {
	case "application", "activity", "activity-alias", "service", "provider", "receiver":
	default:
		return
	}
	launcherTag := tag == "application" || tag == "activity" || tag == "activity-alias"
	for _, a := range start.Attr {
		if a.Name.Space != androidResNamespace || (a.Name.Local != "icon" && a.Name.Local != "roundIcon") {
			continue
		}
		ref, ok := manifestIconRef(a.Value)
		if !ok {
			continue
		}
		if launcherTag {
			m.launcher[ref] = true
		}
		if a.Name.Local == "roundIcon" {
			m.round[ref] = true
		}
	}
}

func manifestIconRef(value string) (string, bool) {
	switch {
	case strings.HasPrefix(value, "@drawable/"):
		return strings.TrimPrefix(value, "@drawable/"), true
	case strings.HasPrefix(value, "@mipmap/"):
		return value, true
	}
	return "", false
}
