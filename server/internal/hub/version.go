package hub
import (
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)
func Version() string {
	versionOnce.Do(func() { version = readVersion(time.Now()) })
	return version
}
var (
	versionOnce sync.Once
	version     string
)
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
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified == "true" {
		revision += "-dirty"
	}
	return revision
}
