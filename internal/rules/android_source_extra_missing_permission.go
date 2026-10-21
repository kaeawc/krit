package rules

import (
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

type permissionAPI struct {
	api             string
	callee          string
	requirement     missingPermissionRequirement
	callTargets     []string
	receiverTypes   []string
	staticReceivers []string
}

// missingPermissionRequirementKind mirrors AOSP Lint's PermissionRequirement /
// PermissionHolder model: a @RequiresPermission may name a single permission,
// require all of a set (allOf), or accept any one of a set (anyOf). Each
// flavor has distinct guard-coverage semantics.
type missingPermissionRequirementKind int

const (
	missingPermissionRequirementSingle missingPermissionRequirementKind = iota
	missingPermissionRequirementAnyOf
	missingPermissionRequirementAllOf
)

type missingPermissionRequirement struct {
	kind  missingPermissionRequirementKind
	perms []string
}

func singlePermRequirement(perm string) missingPermissionRequirement {
	return missingPermissionRequirement{kind: missingPermissionRequirementSingle, perms: []string{perm}}
}

func (r missingPermissionRequirement) empty() bool { return len(r.perms) == 0 }

// describe renders the requirement for the diagnostic message.
func (r missingPermissionRequirement) describe() string {
	if len(r.perms) == 0 {
		return ""
	}
	if len(r.perms) == 1 {
		return r.perms[0]
	}
	sep := " and "
	if r.kind == missingPermissionRequirementAnyOf {
		sep = " or "
	}
	return strings.Join(r.perms, sep)
}

// messageSuffix returns "permission" or "permissions" to match the requirement
// arity in the diagnostic.
func (r missingPermissionRequirement) messageSuffix() string {
	if len(r.perms) > 1 {
		return "permissions"
	}
	return "permission"
}

// satisfied reports whether the requirement is fully met given a predicate
// that answers whether a specific permission is guarded at the call site.
// anyOf is satisfied by any guarded perm; allOf requires every perm guarded.
func (r missingPermissionRequirement) satisfied(has func(perm string) bool) bool {
	if len(r.perms) == 0 {
		return false
	}
	if r.kind == missingPermissionRequirementAllOf {
		for _, p := range r.perms {
			if !has(p) {
				return false
			}
		}
		return true
	}
	for _, p := range r.perms {
		if has(p) {
			return true
		}
	}
	return false
}

// covers reports whether holding this requirement (e.g. on an enclosing
// function) guarantees the caller holds the given specific permission.
// allOf=[A,B] covers A and B; single P covers P; anyOf never covers a
// specific permission because the actual held perm is unknown.
func (r missingPermissionRequirement) covers(perm string) bool {
	if perm == "" || len(r.perms) == 0 || r.kind == missingPermissionRequirementAnyOf {
		return false
	}
	for _, p := range r.perms {
		if p == perm {
			return true
		}
	}
	return false
}

var missingPermissionAPIs = []permissionAPI{
	{
		api:           "requestLocationUpdates",
		callee:        "requestLocationUpdates",
		requirement:   singlePermRequirement("ACCESS_FINE_LOCATION"),
		callTargets:   []string{"android.location.LocationManager.requestLocationUpdates"},
		receiverTypes: []string{"android.location.LocationManager"},
	},
	{
		api:           "getLastKnownLocation",
		callee:        "getLastKnownLocation",
		requirement:   singlePermRequirement("ACCESS_FINE_LOCATION"),
		callTargets:   []string{"android.location.LocationManager.getLastKnownLocation"},
		receiverTypes: []string{"android.location.LocationManager"},
	},
	{
		api:           "getCellLocation",
		callee:        "getCellLocation",
		requirement:   singlePermRequirement("ACCESS_COARSE_LOCATION"),
		callTargets:   []string{"android.telephony.TelephonyManager.getCellLocation"},
		receiverTypes: []string{"android.telephony.TelephonyManager"},
	},
	{
		api:             "Camera.open",
		callee:          "open",
		requirement:     singlePermRequirement("CAMERA"),
		callTargets:     []string{"android.hardware.Camera.open"},
		staticReceivers: []string{"android.hardware.Camera"},
	},
	{
		api:           "setAudioSource",
		callee:        "setAudioSource",
		requirement:   singlePermRequirement("RECORD_AUDIO"),
		callTargets:   []string{"android.media.MediaRecorder.setAudioSource"},
		receiverTypes: []string{"android.media.MediaRecorder"},
	},
}

var missingPermissionAnnotatedIdentifiers = []string{"RequiresPermission"}

var missingPermissionCandidateCallees = missingPermissionCandidateCalleeSet()

func missingPermissionCandidateCalleeSet() map[string]bool {
	out := make(map[string]bool, len(missingPermissionAPIs))
	for _, api := range missingPermissionAPIs {
		if api.callee != "" {
			out[api.callee] = true
		}
	}
	return out
}

func missingPermissionCandidateCalleeNames() []string {
	out := make([]string, 0, len(missingPermissionCandidateCallees))
	for name := range missingPermissionCandidateCallees {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func missingPermissionOracleIdentifiers() []string {
	out := append([]string(nil), missingPermissionCandidateCalleeNames()...)
	out = append(out, missingPermissionAnnotatedIdentifiers...)
	sort.Strings(out)
	return out
}

var missingPermissionKnownPerms = map[string]bool{
	"ACCESS_COARSE_LOCATION": true,
	"ACCESS_FINE_LOCATION":   true,
	"CAMERA":                 true,
	"RECORD_AUDIO":           true,
}

type MissingPermissionRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *MissingPermissionRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a tier-2 (medium) base confidence. The rule requires a
// structural or resolved Android API anchor and a same-permission guard proof,
// but can still run with source-level type evidence when KAA call targets are
// unavailable.
func (r *MissingPermissionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingPermissionRule) check(ctx *api.Context) {
	file := ctx.File
	if first := file.FlatChild(ctx.Idx, 0); first != 0 && file.FlatType(first) == "call_expression" {
		return
	}
	callee := flatCallExpressionName(file, ctx.Idx)
	if !missingPermissionCandidateCallees[callee] && !missingPermissionHasAnnotatedSameFileCandidate(ctx, ctx.Idx, callee) {
		return
	}
	match, evidence, ok := missingPermissionRequiredAPI(ctx, ctx.Idx)
	if !ok {
		return
	}
	confidence, ok := semantics.ConfidenceForEvidence(r.Confidence(), evidence)
	if !ok {
		return
	}
	if missingPermissionHasStructuralGuard(ctx, ctx.Idx, match.requirement) {
		return
	}
	line := file.FlatRow(ctx.Idx) + 1
	col := file.FlatCol(ctx.Idx) + 1
	f := r.Finding(file, line, col,
		match.api+" requires "+match.requirement.describe()+" "+match.requirement.messageSuffix()+". Ensure checkSelfPermission() is called before this API.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func missingPermissionRequiredAPI(ctx *api.Context, call uint32) (permissionAPI, semantics.SemanticEvidence, bool) {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return permissionAPI{}, semantics.EvidenceUnresolved, false
	}
	for i := range missingPermissionAPIs {
		api := missingPermissionAPIs[i]
		if target.CalleeName != api.callee {
			continue
		}
		if target.Resolved && missingPermissionCallTargetMatches(target.QualifiedName, api.callTargets) {
			return api, semantics.EvidenceResolved, true
		}
		if missingPermissionStaticReceiverMatches(ctx.File, target, api.staticReceivers) {
			return api, semantics.EvidenceQualifiedReceiver, true
		}
		if semantics.MatchQualifiedReceiver(ctx, call, api.receiverTypes...) {
			return api, semantics.EvidenceQualifiedReceiver, true
		}
		if missingPermissionReceiverDeclarationHasType(ctx, target.Receiver.Node, api.receiverTypes) {
			return api, semantics.EvidenceSameOwner, true
		}
	}
	if api, ok := missingPermissionAnnotatedSameFileTarget(ctx, call, target); ok {
		return api, semantics.EvidenceSameFileDeclaration, true
	}
	return permissionAPI{}, semantics.EvidenceUnresolved, false
}

func missingPermissionHasAnnotatedSameFileCandidate(ctx *api.Context, call uint32, callee string) bool {
	if ctx.File == nil || call == 0 || callee == "" {
		return false
	}
	file := ctx.File
	if !missingPermissionFileHasRequiresPermissionAnnotation(file) {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || !missingPermissionFunctionHasName(file, fn, callee) {
			return
		}
		if req, ok := missingPermissionRequiredPermissionAnnotation(ctx, fn); ok && !req.empty() && semantics.SameFileDeclarationMatch(ctx, fn, call) {
			found = true
		}
	})
	return found
}

func missingPermissionFunctionHasName(file *scanner.File, fn uint32, name string) bool {
	if file == nil || fn == 0 || name == "" {
		return false
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeString(child, nil) == name {
			return true
		}
	}
	return false
}

func missingPermissionCallTargetMatches(got string, wants []string) bool {
	got = strings.ReplaceAll(strings.TrimSpace(got), "#", ".")
	for _, want := range wants {
		want = strings.ReplaceAll(strings.TrimSpace(want), "#", ".")
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func missingPermissionStaticReceiverMatches(file *scanner.File, target semantics.CallTarget, receivers []string) bool {
	if file == nil || !target.Receiver.Valid() || len(receivers) == 0 {
		return false
	}
	receiverPath := missingPermissionIdentifierPath(file, target.Receiver.Node)
	for _, receiver := range receivers {
		if receiverPath == receiver {
			return true
		}
		if receiverPath == missingPermissionSimpleName(receiver) && missingPermissionHasImport(file, receiver) {
			return true
		}
	}
	return false
}

func missingPermissionReceiverDeclarationHasType(ctx *api.Context, receiver uint32, types []string) bool {
	if ctx.File == nil || receiver == 0 || len(types) == 0 {
		return false
	}
	file := ctx.File
	name := semantics.ReferenceName(file, receiver)
	if name == "" {
		return false
	}
	for owner, ok := file.FlatParent(receiver); ok; owner, ok = file.FlatParent(owner) {
		switch file.FlatType(owner) {
		case "function_declaration", "secondary_constructor", "class_declaration", "object_declaration", "source_file":
			if missingPermissionOwnerDeclaresTypedName(file, owner, name, types) {
				return true
			}
		}
	}
	return false
}

func missingPermissionOwnerDeclaresTypedName(file *scanner.File, owner uint32, name string, types []string) bool {
	found := false
	file.FlatWalkAllNodes(owner, func(node uint32) {
		if found {
			return
		}
		switch file.FlatType(node) {
		case "parameter", "class_parameter", "property_declaration", "variable_declaration":
			if extractIdentifierFlat(file, node) == name && missingPermissionNodeMentionsType(file, node, types) {
				found = true
			}
		}
	})
	return found
}

func missingPermissionNodeMentionsType(file *scanner.File, node uint32, types []string) bool {
	found := false
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "type_identifier", "user_type":
			path := missingPermissionIdentifierPath(file, candidate)
			for _, typ := range types {
				if path == typ || path == missingPermissionSimpleName(typ) && missingPermissionHasImport(file, typ) {
					found = true
					return
				}
			}
		}
	})
	return found
}

func missingPermissionAnnotatedSameFileTarget(ctx *api.Context, call uint32, target semantics.CallTarget) (permissionAPI, bool) {
	if ctx.File == nil || target.CalleeName == "" {
		return permissionAPI{}, false
	}
	file := ctx.File
	var out permissionAPI
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || !semantics.SameFileDeclarationMatch(ctx, fn, call) {
			return
		}
		req, ok := missingPermissionRequiredPermissionAnnotation(ctx, fn)
		if !ok || req.empty() {
			return
		}
		out = permissionAPI{api: target.CalleeName, callee: target.CalleeName, requirement: req}
		found = true
	})
	return out, found
}
