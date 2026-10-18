package oracle

import (
	"encoding/json"
	"strings"
)

// analyzeWithCheckMethod is the krit-fir daemon request that answers the
// oracle's analyzeAll and a rule check from one compilation (#810). A jar that
// predates it answers "Unknown command".
const analyzeWithCheckMethod = "analyzeWithCheck"

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
		rider.abandon()
		return d.AnalyzeAllWithCallFilter(callFilter)
	}
	body, deliver := rider.Prepare()
	if len(body) == 0 || deliver == nil {
		rider.abandon()
		return d.AnalyzeAllWithCallFilter(callFilter)
	}

	d.mu.Lock()
	params := callFilterParams(callFilter)
	if params == nil {
		params = map[string]interface{}{}
	}
	params["sharedCheck"] = json.RawMessage(body)
	resp, err := d.send(analyzeWithCheckMethod, params)
	d.mu.Unlock()
	if err != nil {
		if !isUnknownCommand(err) {
			return nil, err
		}
		// An older jar: the daemon is healthy, it just lacks the request.
		d.noSharedCheck.Store(true)
		// Nothing will come back with this compile: let the caller go on its
		// own while the oracle's plain analysis runs.
		rider.abandon()
		return d.AnalyzeAllWithCallFilter(callFilter)
	}

	_, spelling := d.requestSpelling(nil)
	data, err := unmarshalCallerOracleData(resp.Result, spelling)
	if err != nil {
		return nil, err
	}
	if resp.Check != nil && len(*resp.Check) > 0 {
		deliver(*resp.Check)
	}
	return data, nil
}

// isUnknownCommand reports whether err is krit-fir refusing a request it does
// not implement.
func isUnknownCommand(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Unknown command")
}
