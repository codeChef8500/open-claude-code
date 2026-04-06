package ipc

// MessageType constants for IPC communication between
// supervisor, workers, and external CLI commands.
// Aligned with claude-code-main UDS messaging protocol.
type MessageType string

const (
	// MsgTypeUserInput injects a user message into the worker's conversation.
	MsgTypeUserInput MessageType = "user_input"
	// MsgTypeTickInject sends a proactive tick to the worker.
	MsgTypeTickInject MessageType = "tick_inject"
	// MsgTypeShutdown requests graceful shutdown.
	MsgTypeShutdown MessageType = "shutdown"
	// MsgTypePing is a health check.
	MsgTypePing MessageType = "ping"
	// MsgTypePong is the reply to a ping.
	MsgTypePong MessageType = "pong"
	// MsgTypeStatus requests current worker status.
	MsgTypeStatus MessageType = "status"
	// MsgTypeStatusReply carries the status response.
	MsgTypeStatusReply MessageType = "status_reply"
	// MsgTypeLog streams a log line from the worker.
	MsgTypeLog MessageType = "log"
)
