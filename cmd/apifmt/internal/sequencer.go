package internal

import (
	"context"
	"fmt"
	"go/scanner"
	"io"

	"golang.org/x/sync/semaphore"
)

// A Sequencer performs concurrent tasks that may write output, but emits that
// output in a deterministic order.
type Sequencer struct {
	maxWeight int64
	sem       *semaphore.Weighted   // weighted by input bytes (an approximate proxy for memory overhead)
	prev      <-chan *reporterState // 1-buffered
}

// NewSequencer returns a Sequencer that allows concurrent tasks up to maxWeight
// and writes tasks' output to out and err.
func NewSequencer(maxWeight int64, out, err io.Writer) *Sequencer {
	sem := semaphore.NewWeighted(maxWeight)
	prev := make(chan *reporterState, 1)
	prev <- &reporterState{out: out, err: err}
	return &Sequencer{
		maxWeight: maxWeight,
		sem:       sem,
		prev:      prev,
	}
}

// exclusive is a weight that can be passed to a Sequencer to cause
// a task to be executed without any other concurrent tasks.
const exclusive = -1

// Add blocks until the Sequencer has enough weight to spare, then adds f as a
// task to be executed concurrently.
//
// If the weight is either negative or larger than the Sequencer's maximum
// weight, Add blocks until all other tasks have completed, then the task
// executes exclusively (blocking all other calls to Add until it completes).
//
// f may run concurrently in a goroutine, but its output to the passed-in
// Reporter will be sequential relative to the other tasks in the Sequencer.
//
// If f invokes a method on the Reporter, execution of that method may block
// until the previous task has finished. (To maximize concurrency, f should
// avoid invoking the Reporter until it has finished any parallelizable work.)
//
// If f returns a non-nil error, that error will be reported after f's output
// (if any) and will cause a nonzero final exit code.
func (s *Sequencer) Add(weight int64, f func(*Reporter) error) {
	if weight < 0 || weight > s.maxWeight {
		weight = s.maxWeight
	}
	if err := s.sem.Acquire(context.TODO(), weight); err != nil {
		// Change the task from "execute f" to "report err".
		weight = 0
		f = func(*Reporter) error { return err }
	}

	r := &Reporter{prev: s.prev}
	next := make(chan *reporterState, 1)
	s.prev = next

	// Start f in parallel: it can run until it invokes a method on r, at which
	// point it will block until the previous task releases the output state.
	go func() {
		if err := f(r); err != nil {
			r.Report(err)
		}
		next <- r.getState() // Release the next task.
		s.sem.Release(weight)
	}()
}

// AddReport prints an error to s after the output of any previously-added
// tasks, causing the final exit code to be nonzero.
func (s *Sequencer) AddReport(err error) {
	s.Add(0, func(*Reporter) error { return err })
}

// GetExitCode waits for all previously-added tasks to complete, then returns an
// exit code for the sequence suitable for passing to os.Exit.
func (s *Sequencer) GetExitCode() int {
	c := make(chan int, 1)
	s.Add(0, func(r *Reporter) error {
		c <- r.ExitCode()
		return nil
	})
	return <-c
}

// A Reporter reports output, warnings, and errors.
type Reporter struct {
	prev  <-chan *reporterState
	state *reporterState
}

// reporterState carries the state of a Reporter instance.
//
// Only one Reporter at a time may have access to a reporterState.
type reporterState struct {
	out, err io.Writer
	exitCode int
}

// getState blocks until any prior reporters are finished with the Reporter
// state, then returns the state for manipulation.
func (r *Reporter) getState() *reporterState {
	if r.state == nil {
		r.state = <-r.prev
	}
	return r.state
}

// Warnf emits a warning message to the Reporter's error stream,
// without changing its exit code.
func (r *Reporter) Warnf(format string, args ...interface{}) {
	fmt.Fprintf(r.getState().err, format, args...)
}

// Write emits a slice to the Reporter's output stream.
//
// Any error is returned to the caller, and does not otherwise affect the
// Reporter's exit code.
func (r *Reporter) Write(p []byte) (int, error) {
	return r.getState().out.Write(p)
}

// Report emits a non-nil error to the Reporter's error stream,
// changing its exit code to a nonzero value.
func (r *Reporter) Report(err error) {
	if err == nil {
		panic("Report with nil error")
	}
	st := r.getState()
	scanner.PrintError(st.err, err)
	st.exitCode = 2
}

func (r *Reporter) ExitCode() int {
	return r.getState().exitCode
}
