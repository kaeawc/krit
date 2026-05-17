//go:build linux

package serve

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

// errFanotifyUnsupported is returned when the kernel rejects
// fanotify_init with the flags this backend needs. The dispatcher
// logs and falls back to fsnotify.
var errFanotifyUnsupported = errors.New("fanotify backend unsupported by this kernel")

// fanotifyBackend marks the filesystem containing root with a single
// FAN_MARK_FILESYSTEM mark and reports file-level CREATE / MODIFY /
// DELETE / MOVE events as backendEvents whose Path is rooted at
// rootAbs. The kernel sees every event on the filesystem (this is
// the efficiency win versus the per-directory inotify watches that
// fsnotify uses); the backend filters down to events whose resolved
// path is under root.
//
// Event reporting relies on FAN_REPORT_DFID_NAME (Linux 5.17+): each
// event arrives without an fd and instead carries the parent
// directory's file_handle plus the entry name. We resolve via
// open_by_handle_at + readlink /proc/self/fd/N. Falls back with
// errFanotifyUnsupported on older kernels so the dispatcher can use
// fsnotify.
//
// Threading model: the read goroutine owns fd + mountFd. Close
// signals via the self-pipe wakeFd so the blocking poll returns
// promptly without depending on the kernel to wake threads sleeping
// on a closed fd (it doesn't, reliably).
type fanotifyBackend struct {
	fd      int
	mountFd int
	rootAbs string
	// rootPrefix is rootAbs + filepath.Separator, precomputed once
	// so underRoot's hot-path filter doesn't call filepath.Clean on
	// every event. Paths from resolveDFIDName are already cleaned
	// via filepath.Join, so a HasPrefix is the only check needed.
	rootPrefix string

	// wakeFd[0] is the read end the goroutine polls; wakeFd[1] is the
	// write end Close() pings to break out of poll(). Non-blocking on
	// both ends so a stuck reader / writer can never deadlock teardown.
	wakeFd [2]int

	events chan backendEvent
	errors chan error

	closeOnce sync.Once
	closed    atomic.Bool
	done      chan struct{}
}

// newFanotifyBackend initializes fanotify with FAN_REPORT_DFID_NAME +
// FAN_NONBLOCK, marks the filesystem containing root, and starts the
// read goroutine. Returns errFanotifyUnsupported if the kernel doesn't
// understand the report flags or refuses the mark with EPERM (which
// happens without CAP_SYS_ADMIN).
func newFanotifyBackend(root string) (watcherBackend, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("fanotify: resolve root: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	initFlags := uint(unix.FAN_CLASS_NOTIF | unix.FAN_REPORT_DFID_NAME | unix.FAN_NONBLOCK | unix.FAN_CLOEXEC)
	fd, err := unix.FanotifyInit(initFlags, uint(unix.O_RDONLY|unix.O_CLOEXEC|unix.O_LARGEFILE))
	if err != nil {
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOSYS) {
			return nil, errFanotifyUnsupported
		}
		if errors.Is(err, syscall.EPERM) {
			return nil, fmt.Errorf("%w: need CAP_SYS_ADMIN", errFanotifyUnsupported)
		}
		return nil, fmt.Errorf("fanotify_init: %w", err)
	}

	mask := uint64(unix.FAN_CREATE | unix.FAN_MODIFY | unix.FAN_DELETE |
		unix.FAN_MOVED_FROM | unix.FAN_MOVED_TO | unix.FAN_ONDIR)
	markFlags := uint(unix.FAN_MARK_ADD | unix.FAN_MARK_FILESYSTEM)
	if err := unix.FanotifyMark(fd, markFlags, mask, unix.AT_FDCWD, rootAbs); err != nil {
		_ = unix.Close(fd)
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EPERM) {
			return nil, fmt.Errorf("%w: %w", errFanotifyUnsupported, err)
		}
		return nil, fmt.Errorf("fanotify_mark %s: %w", rootAbs, err)
	}

	// mountFd is the "anchor" handle for open_by_handle_at. Any fd on
	// the same mount works; the root dir is the obvious choice. We
	// open as a real RDONLY directory rather than O_PATH because
	// some kernels reject O_PATH fds as the mount_fd argument with
	// EBADF.
	mountFd, err := unix.Open(rootAbs, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("fanotify: open root %s: %w", rootAbs, err)
	}

	wake := [2]int{}
	if err := unix.Pipe2(wake[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		_ = unix.Close(fd)
		_ = unix.Close(mountFd)
		return nil, fmt.Errorf("fanotify: self-pipe: %w", err)
	}

	b := &fanotifyBackend{
		fd:         fd,
		mountFd:    mountFd,
		rootAbs:    rootAbs,
		rootPrefix: rootAbs + string(filepath.Separator),
		wakeFd:     wake,
		// 64 is sized for burst tolerance: editors like JetBrains
		// and VS Code can emit ~30 events in a single save dance.
		// fsnotify uses 16; fanotify needs more headroom because
		// FAN_MARK_FILESYSTEM also sees mount-wide noise that the
		// underRoot filter drops, but only after the kernel has
		// already delivered it to userspace.
		events: make(chan backendEvent, 64),
		errors: make(chan error, 4),
		done:   make(chan struct{}),
	}
	go b.read()
	return b, nil
}

// Add is a no-op for fanotify: the filesystem mark already covers
// every directory under the root. Returning nil keeps the recursive-
// walk caller in fileWatcher happy without special-casing the
// backend kind in the watcher core.
func (b *fanotifyBackend) Add(string) error { return nil }

func (b *fanotifyBackend) Close() error {
	var ret error
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		// Wake the poll() loop so the goroutine can exit before we
		// close the fanotify fd. A single byte is enough — the read
		// loop just needs to know to re-check closed.
		_, _ = unix.Write(b.wakeFd[1], []byte{0})
		<-b.done
		if err := unix.Close(b.fd); err != nil {
			ret = err
		}
		_ = unix.Close(b.mountFd)
		_ = unix.Close(b.wakeFd[0])
		_ = unix.Close(b.wakeFd[1])
	})
	return ret
}

func (b *fanotifyBackend) Events() <-chan backendEvent { return b.events }
func (b *fanotifyBackend) Errors() <-chan error        { return b.errors }

// read is the goroutine that drains the fanotify fd. It blocks in
// poll() until either the fanotify fd has data or the self-pipe is
// pinged by Close(). When data is available we read as much as the
// kernel will give us and parse a stream of fanotify_event_metadata
// records.
func (b *fanotifyBackend) read() {
	defer close(b.done)
	defer close(b.events)
	defer close(b.errors)

	buf := make([]byte, 16*1024)
	pfds := []unix.PollFd{
		{Fd: int32(b.fd), Events: unix.POLLIN},
		{Fd: int32(b.wakeFd[0]), Events: unix.POLLIN},
	}
	for {
		if b.closed.Load() {
			return
		}
		// -1 timeout = block until ready. We don't need a periodic
		// wakeup because Close() pings the self-pipe.
		if _, err := unix.Poll(pfds, -1); err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			b.errors <- fmt.Errorf("fanotify poll: %w", err)
			return
		}
		if pfds[1].Revents&unix.POLLIN != 0 || b.closed.Load() {
			// Drain the pipe so a future Close() write doesn't fill
			// the kernel buffer — defensive; one byte usually.
			var sink [16]byte
			for {
				if _, err := unix.Read(b.wakeFd[0], sink[:]); err != nil {
					break
				}
			}
			return
		}
		if pfds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		n, err := unix.Read(b.fd, buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				continue
			}
			if errors.Is(err, syscall.EBADF) {
				return
			}
			b.errors <- fmt.Errorf("fanotify read: %w", err)
			continue
		}
		if n <= 0 {
			continue
		}
		b.parseBatch(buf[:n])
	}
}

// fanotify_event_metadata layout (matches FAN_EVENT_METADATA_LEN = 24):
//
//	__u32 event_len;     // 0..4   total bytes including info records
//	__u8  vers;          // 4
//	__u8  reserved;      // 5
//	__u16 metadata_len;  // 6..8
//	__u64 mask;          // 8..16  aligned
//	__s32 fd;            // 16..20 FAN_NOFD with FAN_REPORT_*
//	__s32 pid;           // 20..24
const fanMetaSize = 24

// parseBatch walks one buffer worth of fanotify events. Each record
// is event_len bytes; bad lengths or truncated records break us out
// of the buffer to avoid feeding garbage to open_by_handle_at.
//
// FAN_Q_OVERFLOW is checked first because the kernel emits it with
// fd=FAN_NOFD and no info records — passing it through handleEvent
// would silently drop it. Surfacing through diag() turns a class of
// missed-event bugs into a visible diagnostic; the watcher's mtime
// sweep (see fileWatcher.sweepOnce equivalents) is the recovery
// mechanism.
func (b *fanotifyBackend) parseBatch(buf []byte) {
	for len(buf) >= fanMetaSize {
		eventLen := binary.LittleEndian.Uint32(buf[0:4])
		if eventLen < fanMetaSize || int(eventLen) > len(buf) {
			return
		}
		metaLen := binary.LittleEndian.Uint16(buf[6:8])
		mask := binary.LittleEndian.Uint64(buf[8:16])
		if mask&unix.FAN_Q_OVERFLOW != 0 {
			b.diag("event queue overflow — kernel dropped events; some cache invalidations may be missed")
			buf = buf[eventLen:]
			continue
		}
		if metaLen < fanMetaSize || uint32(metaLen) > eventLen {
			buf = buf[eventLen:]
			continue
		}
		info := buf[metaLen:eventLen]
		b.handleEvent(mask, info)
		buf = buf[eventLen:]
	}
}

// fanotify_event_info_header is 4 bytes:
//
//	__u8  info_type;  // FAN_EVENT_INFO_TYPE_*
//	__u8  pad;
//	__u16 len;        // total bytes including header
const fanInfoHeaderSize = 4

// handleEvent extracts the DFID_NAME info record from one event's
// trailing bytes, resolves it to an absolute path under rootAbs, and
// pushes a backendEvent. Events without a usable DFID_NAME record
// are dropped — they're either FID-only (older kernel report
// shape) or events on the mount root itself, neither of which the
// daemon cares about.
func (b *fanotifyBackend) handleEvent(mask uint64, info []byte) {
	sawDFID := false
	for len(info) >= fanInfoHeaderSize {
		infoType := info[0]
		recLen := binary.LittleEndian.Uint16(info[2:4])
		if recLen < fanInfoHeaderSize || int(recLen) > len(info) {
			return
		}
		rec := info[:recLen]
		info = info[recLen:]
		if infoType != unix.FAN_EVENT_INFO_TYPE_DFID_NAME &&
			infoType != unix.FAN_EVENT_INFO_TYPE_NEW_DFID_NAME &&
			infoType != unix.FAN_EVENT_INFO_TYPE_OLD_DFID_NAME {
			b.diag("info_type %d skipped (len=%d)", infoType, recLen)
			continue
		}
		sawDFID = true
		path, ok := b.resolveDFIDName(rec)
		if !ok {
			continue
		}
		if !b.underRoot(path) {
			b.diag("event outside root: %s", path)
			continue
		}
		b.events <- backendEvent{Path: path, Op: opFromFanotifyMask(mask)}
	}
	if !sawDFID {
		b.diag("event mask=0x%x had no DFID_NAME record (len=%d)", mask, len(info))
	}
}

// diag emits a non-fatal diagnostic to the errors channel. Used
// when an event arrives in a shape the parser can't make sense of —
// surfaces unparseable events to the integration test rather than
// silently dropping them. Drops if the errors channel is full so a
// flood can't deadlock the read goroutine.
func (b *fanotifyBackend) diag(format string, args ...any) {
	select {
	case b.errors <- fmt.Errorf("fanotify: "+format, args...):
	default:
	}
}

// resolveDFIDName parses a FAN_EVENT_INFO_TYPE_DFID_NAME record and
// resolves the contained directory file_handle + name to an absolute
// path via open_by_handle_at + /proc/self/fd readlink.
//
// Record layout after the 4-byte header:
//
//	__kernel_fsid_t fsid;        // 8 bytes (two int32)
//	struct file_handle {
//	    __u32 handle_bytes;
//	    int   handle_type;
//	    unsigned char f_handle[handle_bytes];
//	}
//	char name[];                 // null-terminated, padded to align
func (b *fanotifyBackend) resolveDFIDName(rec []byte) (string, bool) {
	const fsidSize = 8
	if len(rec) < fanInfoHeaderSize+fsidSize+8 {
		return "", false
	}
	off := fanInfoHeaderSize + fsidSize
	handleBytes := binary.LittleEndian.Uint32(rec[off : off+4])
	handleType := int32(binary.LittleEndian.Uint32(rec[off+4 : off+8]))
	off += 8
	if off+int(handleBytes) > len(rec) {
		return "", false
	}
	// NewFileHandle copies handle bytes into its own buffer, so we
	// pass the slice directly — no need for an intermediate copy.
	handleData := rec[off : off+int(handleBytes)]
	off += int(handleBytes)

	// Name follows the handle data, null-terminated.
	name := ""
	if off < len(rec) {
		end := off
		for end < len(rec) && rec[end] != 0 {
			end++
		}
		name = string(rec[off:end])
	}

	fh := unix.NewFileHandle(handleType, handleData)
	dirFd, err := unix.OpenByHandleAt(b.mountFd, fh, unix.O_PATH|unix.O_CLOEXEC)
	if err != nil {
		// ESTALE: the directory was unlinked between event emission
		// and resolution. EACCES / EPERM happen in containers with
		// stricter security — both are recoverable: drop the event,
		// next analyze rehashes from disk anyway.
		b.diag("open_by_handle_at: %v (handle_bytes=%d type=%d name=%q)", err, handleBytes, handleType, name)
		return "", false
	}
	defer unix.Close(dirFd)

	dirPath, err := readlinkFd(dirFd)
	if err != nil || dirPath == "" {
		return "", false
	}
	if name == "" || name == "." {
		return filepath.Clean(dirPath), true
	}
	return filepath.Join(dirPath, name), true
}

// readlinkFd reads /proc/self/fd/<fd> to recover the canonical path
// of an open directory fd. We grow the buffer until the kernel
// reports it isn't truncated. A bare PATH_MAX-sized buffer would do
// in practice but the slight cost of one Readlink retry on the rare
// long-path case beats a hard-coded ceiling.
func readlinkFd(fd int) (string, error) {
	link := "/proc/self/fd/" + strconv.Itoa(fd)
	buf := make([]byte, 256)
	for {
		n, err := unix.Readlink(link, buf)
		if err != nil {
			return "", err
		}
		if n < len(buf) {
			return string(buf[:n]), nil
		}
		buf = make([]byte, len(buf)*2)
	}
}

// underRoot reports whether path is inside the watched root. Because
// FAN_MARK_FILESYSTEM gives us every event on the mount, the backend
// must filter — otherwise editing an unrelated file on the same
// volume would invalidate Krit's caches. Paths reaching this
// function are already cleaned (resolveDFIDName uses filepath.Join /
// filepath.Clean), so the hot path skips a redundant Clean.
func (b *fanotifyBackend) underRoot(path string) bool {
	if path == b.rootAbs {
		return true
	}
	return strings.HasPrefix(path, b.rootPrefix)
}

// opFromFanotifyMask translates the FAN_* mask into our backendOp
// bitmask. The watcher's handle() treats Create and Write
// interchangeably for invalidation, but keeping the distinction lets
// the new-directory addRecursive branch fire correctly on FAN_CREATE
// for a fresh directory. Even though fanotify's filesystem mark
// already covers the new directory, the watcher's existing code
// path stays uniform.
func opFromFanotifyMask(mask uint64) backendOp {
	var op backendOp
	if mask&unix.FAN_CREATE != 0 || mask&unix.FAN_MOVED_TO != 0 {
		op |= opCreate
	}
	if mask&unix.FAN_MODIFY != 0 {
		op |= opWrite
	}
	if mask&unix.FAN_DELETE != 0 || mask&unix.FAN_MOVED_FROM != 0 {
		op |= opRemove
	}
	if mask&(unix.FAN_MOVED_FROM|unix.FAN_MOVED_TO) != 0 {
		op |= opRename
	}
	return op
}
