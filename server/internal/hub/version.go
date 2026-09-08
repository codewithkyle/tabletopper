package hub

import (
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

// Version is the build this process is running, and it is one string doing two
// jobs: it rides in every snapshot event, and it is the ?v= on the room
// bundle's URL.
//
// THOSE TWO USES ARE THE WHOLE DESIGN. A client that reconnects after a deploy
// receives a snapshot whose version differs from the one it started with and
// reloads itself; the reload asks for a bundle URL that has never been fetched,
// so the one-hour cache on /static/ cannot serve it the old script. Without the
// second half the first half would reload a page into the same stale
// JavaScript, forever.
//
// IT IS THE VCS REVISION WHERE THERE IS ONE. -buildvcs defaults to auto, so a
// build from inside a git work tree carries vcs.revision and vcs.modified in
// its build info. A build that has neither falls back to this process's start
// time, which is exactly right for development: every restart is a new version,
// so every restart reloads the tab.
//
// THE CONTAINER TAKES THE FALLBACK TODAY, and that is worth knowing rather than
// discovering. The image is built from a COPY of ./server and the repository's
// .git directory is not in it, so the toolchain has no revision to stamp and
// every container start is a new version -- which reloads every client that
// reconnects to it, correct but heavier than it needs to be. Putting the real
// revision in is a build argument and a -ldflags -X, which is a change to the
// deploy rather than to this file.
func Version() string {
	versionOnce.Do(func() { version = readVersion(time.Now()) })

	return version
}

var (
	versionOnce sync.Once
	version     string
)

// readVersion is Version with the fallback injected, so a test can pin it.
func readVersion(start time.Time) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev-" + strconv.FormatInt(start.UnixMilli(), 36)
	}

	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return "dev-" + strconv.FormatInt(start.UnixMilli(), 36)
	}

	// Twelve characters is what a person reads in a log line and still pastes
	// into `git show`. The full forty adds nothing to a comparison that is only
	// ever equality.
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified == "true" {
		revision += "-dirty"
	}

	return revision
}
