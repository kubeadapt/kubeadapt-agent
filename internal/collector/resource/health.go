package resource

// informerHealthy checks whether an informer-based collector is still running.
// It returns false if the informer's Run goroutine has exited (done is closed)
// without being asked to stop (stopCh is still open), which indicates a crash.
func informerHealthy(stopCh, done <-chan struct{}) (healthy bool, reason string) {
	select {
	case <-done:
		// The informer goroutine has exited. Check if we asked it to.
		select {
		case <-stopCh:
			// We closed stopCh — this is a graceful shutdown, still "healthy".
			return true, ""
		default:
			// stopCh is open but done is closed — informer crashed.
			return false, "informer goroutine exited unexpectedly"
		}
	default:
		// Informer goroutine is still running.
		return true, ""
	}
}
