package ccr

// WorkerState describes the current state of a CCR worker.
// Aligned with claude-code-main ccrClient.ts WorkerState.
type WorkerState string

const (
	WorkerStateIdle       WorkerState = "idle"
	WorkerStateProcessing WorkerState = "processing"
	WorkerStateSleeping   WorkerState = "sleeping"
)

// DeliveryStatus describes the outcome of an event delivery attempt.
type DeliveryStatus string

const (
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusDropped   DeliveryStatus = "dropped"
)

// DeliveryReport is sent back to the CCR backend to confirm event delivery.
type DeliveryReport struct {
	EventID string         `json:"event_id"`
	Status  DeliveryStatus `json:"status"`
}
