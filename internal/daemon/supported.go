package daemon

import "errors"

// ErrUnsupported is returned wherever daemon mode is requested on a
// platform that cannot run it (Windows).
var ErrUnsupported = errors.New("daemon mode is not supported on Windows")
