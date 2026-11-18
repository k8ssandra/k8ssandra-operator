package medusa

// TaskConcurrency is the value of MedusaTaskConcurrencyAnnotation.
type TaskConcurrency string

const (
	// TaskConcurrencyAll runs the task on every pod of the datacenter at the same time. It is the default.
	TaskConcurrencyAll TaskConcurrency = "all"
	// TaskConcurrencyRack runs the task on every pod of one rack at the same time, one rack after another.
	TaskConcurrencyRack TaskConcurrency = "rack"
	// TaskConcurrencyPod runs the task on one pod at a time.
	TaskConcurrencyPod TaskConcurrency = "pod"
)

// ParseTaskConcurrency returns the concurrency for an annotation value. An empty value is TaskConcurrencyAll. Any
// other unknown value returns false, so the caller can fall back to the most conservative mode.
func ParseTaskConcurrency(value string) (TaskConcurrency, bool) {
	switch TaskConcurrency(value) {
	case "", TaskConcurrencyAll:
		return TaskConcurrencyAll, true
	case TaskConcurrencyRack:
		return TaskConcurrencyRack, true
	case TaskConcurrencyPod:
		return TaskConcurrencyPod, true
	default:
		return TaskConcurrencyPod, false
	}
}
