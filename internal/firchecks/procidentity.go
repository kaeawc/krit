package firchecks

import (
	"encoding/json"
	"os"

	"github.com/kaeawc/krit/internal/procstart"
)

// firProcessStart returns a token naming this incarnation of pid: the kernel
// start time. A PID reused by an unrelated process after a crash or reboot
// carries a different token, so a recorded token proves the process is the
// one that was registered. It returns "" when the start time cannot be read
// (unsupported platform, ps missing, process gone); callers must then treat
// the process as unproven and never signal it.
//
// It is a variable so tests can simulate platforms without a readable start
// time.
var firProcessStart = procstart.Of

// firPIDProven reports whether pid is the process the registry recorded,
// by comparing the start time stored at publication with the live one. A
// record without a start time (written before it was recorded, or on a
// platform that cannot read it) is unproven.
func firPIDProven(meta firAOTMeta, pid int) bool {
	if meta.PID != pid || meta.StartTime == "" {
		return false
	}
	return firProcessStart(pid) == meta.StartTime
}

// readFirAOTMeta loads the record's metadata; ok is false when it is missing
// or unreadable.
func readFirAOTMeta(sourcesHash string) (firAOTMeta, bool) {
	var meta firAOTMeta
	data, err := os.ReadFile(firAOTPath(sourcesHash))
	if err != nil || json.Unmarshal(data, &meta) != nil {
		return firAOTMeta{}, false
	}
	return meta, true
}
