package daemon

import (
	"fmt"
	"sync"
)

// WorkerFactory creates and runs a worker of a given kind. It should block
// until the worker is done or ctx is cancelled via the WorkerConfig.
type WorkerFactory func(cfg WorkerConfig) error

var (
	registryMu      sync.RWMutex
	workerFactories = make(map[WorkerKind]WorkerFactory)
)

// RegisterWorkerKind registers a factory for a worker kind.
// Typically called in an init() function.
func RegisterWorkerKind(kind WorkerKind, factory WorkerFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	workerFactories[kind] = factory
}

// RunDaemonWorker dispatches to the registered factory for the given kind.
// Called from the CLI fast-path: agent-engine --daemon-worker=<kind>.
func RunDaemonWorker(kind string) error {
	registryMu.RLock()
	factory, ok := workerFactories[WorkerKind(kind)]
	registryMu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown worker kind: %q", kind)
	}

	cfg := WorkerConfig{
		Kind:        WorkerKind(kind),
		SessionKind: "daemon-worker",
	}
	return factory(cfg)
}

// RegisteredWorkerKinds returns all registered worker kinds.
func RegisteredWorkerKinds() []WorkerKind {
	registryMu.RLock()
	defer registryMu.RUnlock()
	kinds := make([]WorkerKind, 0, len(workerFactories))
	for k := range workerFactories {
		kinds = append(kinds, k)
	}
	return kinds
}
