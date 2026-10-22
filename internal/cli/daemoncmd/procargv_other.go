//go:build !linux && !darwin

package daemoncmd

import "errors"

// processArgv cannot inspect other processes here, so stop refuses to
// signal unless the daemon itself confirmed its pid over the socket.
func processArgv(int) ([]string, error) {
	return nil, errors.New("process inspection is not supported on this platform")
}
