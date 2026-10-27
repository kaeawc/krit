package oracle

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// analyzeWithCheckMethod is the krit-fir daemon request that answers the
// oracle's analyzeAll and a rule check from one compilation (#810). A jar that
// predates it answers "Unknown command".
const analyzeWithCheckMethod = "analyzeWithCheck"

// sharedCheckKey is the params key the combined request carries the check
// request under.
const sharedCheckKey = "kritSharedCheck"

// AnalyzeAllWithCheck is AnalyzeAllWithCallFilter that also asks the daemon to
// run the rider's check request on the same compilation, instead of leaving
// the caller to compile the module a second time.
//
// The rider's response is delivered only when krit-fir shared the compile.
// Anything else leaves the rider undelivered and the result is the oracle's
// alone, exactly as AnalyzeAllWithCallFilter returns it: a nil rider, a jar
// that does not know the request (remembered, so later calls skip it), or a
// compile krit-fir could not share.
func (d *Daemon) AnalyzeAllWithCheck(callFilter *CallTargetFilterSummary, rider *CheckRider) (*Data, error) {
	if rider == nil || rider.Prepare == nil {
		return d.AnalyzeAllWithCallFilter(callFilter)
	}
	if d.noSharedCheck.Load() {
		rider.decline("krit-fir does not carry the check")
		return d.AnalyzeAllWithCallFilter(callFilter)
	}
	body, deliver := rider.Prepare()
	body = withoutCommand(body)
	if len(body) == 0 || deliver == nil {
		rider.decline("nothing to check")
		return d.AnalyzeAllWithCallFilter(callFilter)
	}

	d.mu.Lock()
	if rider.Cancelled != nil && rider.Cancelled() {
		// The pass gave up while this request queued behind another one: it
		// compiles on its own, so carrying its check would be wasted work.
		d.mu.Unlock()
		return d.AnalyzeAllWithCallFilter(callFilter)
	}
	params := callFilterParams(callFilter)
	if params == nil {
		params = map[string]interface{}{}
	}
	params[sharedCheckKey] = json.RawMessage(body)
	if rider.Sent != nil {
		rider.Sent()
	}
	resp, err := d.sendCarrier(params)
	d.mu.Unlock()
	if err != nil {
		if !isUnknownCommand(err) {
			return nil, err
		}
		return d.unsupportedCarrier(callFilter, rider)
	}
	if resp.Result == nil {
		// An older jar that misread the request as something else answers
		// without an oracle result: never trust it with the check again.
		return d.unsupportedCarrier(callFilter, rider)
	}

	_, spelling := d.requestSpelling(nil)
	data, err := unmarshalCallerOracleData(resp.Result, spelling)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.Check == nil || len(*resp.Check) == 0:
		rider.decline("krit-fir did not share the compile")
	case d.aotCachePath != "" && !sharedCheckVerified(*resp.Check):
		// See sharedCheckVerified: a JVM launched from an AOT cache shares a
		// check only once its own canary passed.
		rider.decline("AOT-launched krit-fir did not verify the shared check")
	default:
		deliver(*resp.Check)
	}
	return data, nil
}

// unsupportedCarrier remembers that this daemon's jar cannot carry a check,
// releases the rider's waiter before the plain compile starts, and returns the
// oracle's plain analysis.
func (d *Daemon) unsupportedCarrier(callFilter *CallTargetFilterSummary, rider *CheckRider) (*Data, error) {
	d.noSharedCheck.Store(true)
	rider.decline("krit-fir does not carry the check")
	return d.AnalyzeAllWithCallFilter(callFilter)
}

// withoutCommand drops the embedded request's top-level "command" key. The
// carrier's own "method" names the request; a jar that reads fields without
// regard to nesting would otherwise prefer the embedded "command" and run the
// check instead of refusing the unknown method. The new jar restores it.
func withoutCommand(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil
	}
	delete(fields, "command")
	out, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return bytes.TrimSpace(out)
}

// isUnknownCommand reports whether err is krit-fir refusing this request.
func isUnknownCommand(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Unknown command: "+analyzeWithCheckMethod)
}

// DaemonRequestTimeout is how long one daemon request may take
// (KRIT_TYPES_REQUEST_TIMEOUT, default 10 minutes) before the daemon is killed.
func DaemonRequestTimeout() time.Duration { return daemonRequestTimeout() }

// sendCarrier is send for the combined request, with one difference in the
// circuit breaker's books: an older jar refusing the request as an unknown
// command is an expected answer from a healthy daemon, not a failure, and must
// not push the breaker toward opening (the plain analyzeAll that follows would
// then be refused). Every other error counts as usual. Caller holds d.mu.
func (d *Daemon) sendCarrier(params map[string]interface{}) (*daemonResponse, error) {
	if !d.started {
		return nil, errors.New("daemon not started")
	}
	if !d.breakerAdmit() {
		return nil, errBreakerOpen
	}
	resp, err := d.sendOnce(analyzeWithCheckMethod, params)
	if isUnknownCommand(err) {
		d.breakerRecord(nil)
	} else {
		d.breakerRecord(err)
	}
	return resp, err
}
