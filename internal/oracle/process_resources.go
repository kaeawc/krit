package oracle

import (
	"os"
	"reflect"
	"runtime"

	"github.com/kaeawc/krit/internal/perf"
)

type oracleProcessResult struct {
	OutputPath string
	PeakRSSMB  int64
}

type oracleProcessWaitResult struct {
	Err       error
	PeakRSSMB int64
}

func processStatePeakRSSMB(ps *os.ProcessState) int64 {
	if ps == nil {
		return 0
	}
	usage := ps.SysUsage()
	if usage == nil {
		return 0
	}
	v := reflect.ValueOf(usage)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0
	}
	field := v.FieldByName("Maxrss")
	if !field.IsValid() || !field.CanInt() {
		return 0
	}
	raw := field.Int()
	if raw <= 0 {
		return 0
	}
	bytes := raw * 1024
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		bytes = raw
	}
	const mb = 1024 * 1024
	return (bytes + mb - 1) / mb
}

func addOracleProcessResources(t perf.Tracker, name string, peakRSSMB int64) {
	if peakRSSMB <= 0 {
		return
	}
	addOracleInstant(t, name, map[string]int64{"peakRSSMB": peakRSSMB}, nil)
}
