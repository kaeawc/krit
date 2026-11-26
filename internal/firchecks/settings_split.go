package firchecks

// settings_split.go — recheck the files of source sets whose Kotlin compiler
// settings differ from the merged session's, one session per settings group
// (#1577).

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

// SettingsSplitter rechecks, one FIR session per group of source sets that
// share their compiler settings, every requested file of the groups whose
// settings differ from the merged session's. Its arguments and results are
// those of SourceSetIsolator; files lists every file the pass checks.
type SettingsSplitter func(ctx context.Context, jarPath string, files []string, request []byte) (response []byte, rechecked []string, err error)

// settingsSplitRequestID is the request ID of a settings-group recheck.
const settingsSplitRequestID = 3

// splitOutcome summarizes a settings-group recheck for the verbose log.
type splitOutcome struct {
	ran bool
	// rechecked counts the files the group sessions checked; replaced the
	// verdicts they changed: newlyAuthoritative files the merged session
	// gated, newlyGated files it answered but that miss an opt-in their own
	// source set does not enable, and targetGated files it answered but that
	// fail to compile at their own source set's JVM target.
	rechecked, replaced, newlyAuthoritative, newlyGated, targetGated int
	// kept counts files a group session gated for another reason while the
	// merged session answered them; the merged answer stays.
	kept     int
	duration time.Duration
	err      error
}

// optInError reports whether a gating reason is a missing opt-in: an error
// the real build of that source set reports too, which the merged session
// hid by enabling every source set's opt-ins.
func optInError(reason string) bool {
	return strings.Contains(firstLine(reason), "needs opt-in")
}

// jvmTargetError reports whether a gating reason is an error at the source
// set's own JVM target, such as "Using @JvmRecord is only allowed with
// -jvm-target 16 or later" or "Cannot inline bytecode built with JVM target
// 17 into bytecode that is being built with JVM target 11". The real build
// reports it too; the merged session, compiled at the highest target, does
// not (#1648).
func jvmTargetError(reason string) bool {
	line := strings.ToLower(firstLine(reason))
	return strings.Contains(line, "-jvm-target") || strings.Contains(line, "jvm target")
}

// splitSettings replaces result's verdicts with those of the per-settings
// group sessions. A file a group session answers takes that answer. A file it
// gates keeps the merged verdict unless the merged session gated it too or
// the reason is a missing opt-in or a JVM-target error, so a group session that lacks some context
// can only turn a gated file authoritative, never hide a file the merged
// session answered. Any failure keeps the merged verdicts.
func splitSettings(ctx context.Context, opts PassOptions, requested, sourceDirs, classpath, rules []string, configs RuleConfigs, facts FileFacts, result *Result) (out splitOutcome) {
	began := time.Now()
	defer func() { out.duration = time.Since(began) }()
	jarPath, target := "", CompileTarget{}
	if checker, ok := opts.Checker.(*ProductionFirChecker); ok {
		jarPath, target = checker.JarPath, checker.CompileTarget()
	}
	refs := make([]fileRef, len(requested))
	for i, path := range requested {
		refs[i] = fileRef{Path: path}
	}
	request, spelling, err := encodeCheckRequest(settingsSplitRequestID, refs, sourceDirs, classpath, rules, configs, facts, target)
	if err != nil {
		out.err = err
		return out
	}
	data, rechecked, err := opts.SettingsSplitter(ctx, jarPath, oracle.AbsolutePaths(requested), request)
	if err != nil || len(rechecked) == 0 {
		out.err = err
		return out
	}
	resp, err := decodeCheckResponse(data, settingsSplitRequestID, spelling)
	if err != nil {
		out.err = err
		return out
	}
	if missingKnownFIRRules(rules, resp) {
		out.err = fmt.Errorf("the settings-group check registered no checkers")
		return out
	}
	out.ran = true
	out.rechecked = len(rechecked)
	var files []string
	for _, path := range rechecked {
		path = spelling.Caller(path)
		_, mergedGated := result.ErrorFiles[path]
		_, mergedCrashed := result.Crashed[path]
		mergedAnswered := !mergedGated && !mergedCrashed
		_, groupGated := resp.ErrorFiles[path]
		_, groupCrashed := resp.Crashed[path]
		switch {
		case !groupGated && !groupCrashed:
			if !mergedAnswered {
				out.newlyAuthoritative++
			}
		case !mergedAnswered:
			// Still gated; the group session's reason is the accurate one.
		case groupGated && optInError(errorFileMessage(resp.ErrorFiles, path)):
			out.newlyGated++
		case groupGated && jvmTargetError(errorFileMessage(resp.ErrorFiles, path)):
			out.targetGated++
		default:
			out.kept++
			continue
		}
		files = append(files, path)
	}
	result.replaceFiles(files, resp)
	out.replaced = len(files)
	return out
}

// writeSplitOutcome writes the settings-group recheck summary.
func writeSplitOutcome(w io.Writer, o splitOutcome) {
	fmt.Fprintf(w, "verbose: FIR settings: %s\n", o)
}

func (o splitOutcome) String() string {
	if o.err != nil {
		return fmt.Sprintf("settings-group recheck not applied (%v)", o.err)
	}
	return fmt.Sprintf("rechecked %d files with their own source set's settings in %v: %d gated files now authoritative, %d now gated on a missing opt-in, %d on their own JVM target, %d kept the merged verdict",
		o.rechecked, o.duration.Round(time.Millisecond), o.newlyAuthoritative, o.newlyGated, o.targetGated, o.kept)
}
