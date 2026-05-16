package traces

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// CallerChainDepth is the number of caller frames (under the top
// symbol) the chain hash incorporates. A bounded window keeps two
// states "the same" across deep but irrelevant call differences while
// still distinguishing intentionally different entry paths.
const CallerChainDepth = 4

// callerWindow returns the bounded caller-frame slice
// (frames[1:1+CallerChainDepth]) without copying. Shared between
// HashCallerChain and the reducer so both honor the same window.
func callerWindow(frames []string) []string {
	if len(frames) <= 1 {
		return nil
	}
	end := 1 + CallerChainDepth
	if end > len(frames) {
		end = len(frames)
	}
	return frames[1:end]
}

// HashCallerChain returns a stable hex hash of the top-N caller
// frames under topSymbol. frames is top-of-stack first; the function
// skips frames[0] (that is the top symbol itself) and includes up to
// CallerChainDepth more frames. Empty inputs hash to "".
func HashCallerChain(frames []string) string {
	w := callerWindow(frames)
	if len(w) == 0 {
		return ""
	}
	h := sha256.New()
	for _, f := range w {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

// Fingerprint returns the stable identity of a runtime state.
// Mirrors AutoMobile's ScreenFingerprint construction: a one-way
// hash over the discriminating fields. Two states are "the same"
// iff fingerprints match.
func Fingerprint(topSymbol, callerChainHash string, role RoleTag) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(topSymbol)))
	h.Write([]byte{0})
	h.Write([]byte(callerChainHash))
	h.Write([]byte{0})
	if role == "" {
		role = RoleUnknown
	}
	h.Write([]byte(role))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:12])
}
