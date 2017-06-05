package concurrent

// Request represents a work that Worker needs to execute
//
type Request struct {
	ID          string               // The Id of the worker (for debugging purposes)
	Work        func() error         // The work to be executed by a worker
	ShouldRetry func(err error) bool // The method used by worker to decide whether to retry if execution fails
}
