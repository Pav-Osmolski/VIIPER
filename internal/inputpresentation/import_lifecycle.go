package inputpresentation

import "time"

// ImportLifecycleSource optionally distinguishes a recoverable USB reset from
// final import closure. Both must fence claims and producer leases; only resets
// adopt a successor on the same transport. Implementations must reject stale
// captured generations before changing readiness.
type ImportLifecycleSource interface {
	RetireInputPresentationGenerationForImport(generation uint64, retiredAt time.Time, adoptSuccessor bool) bool
}
