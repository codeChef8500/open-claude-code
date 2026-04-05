package state

// SessionKind constants define the type of session running.
// Aligned with claude-code-main's SessionKind (utils/concurrentSessions.ts).
const (
	SessionKindInteractive  = "interactive"
	SessionKindBG           = "bg"
	SessionKindDaemon       = "daemon"
	SessionKindDaemonWorker = "daemon-worker"
)

// IsDaemonSession returns true if the given session kind is a daemon or daemon-worker.
func IsDaemonSession(kind string) bool {
	return kind == SessionKindDaemon || kind == SessionKindDaemonWorker
}

// IsBackgroundSession returns true if the session kind is a background session.
func IsBackgroundSession(kind string) bool {
	return kind == SessionKindBG
}
