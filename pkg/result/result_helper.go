package result

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// This is just so that we can reference TerminalError in the Godoc of [Error]
var _ error = reconcile.TerminalError(nil)

// ReconcileResult represents the result of a step in the reconciliation process.
//
// We typically split the top-level Reconcile() method of a controller into multiple sub-functions. Each of these
// functions uses ReconcileResult to communicate to its caller how the current iteration should proceed. There are 4
// possible implementations: [Continue], [Done], [RequeueSoon], and [Error].
//
// The idiomatic way to handle a ReconcileResult in an intermediary sub-function is:
//
//	if recResult := callStep1(); recResult.Completed() {
//	   // Global success, requeue or error: stop what we're doing and propagate up
//	   return recResult
//	}
//	// Otherwise, proceed with the next step(s)
//	if recResult := callStep2(); recResult.Completed() {
//	   // etc...
//
// The idiomatic way to handle a ReconcileResult in the top-level Reconcile() method is:
//
//	recResult := callSomeSubmethod()
//	// Possibly inspect the result (e.g. to set an error field in the status)
//	return recResult.Output()
type ReconcileResult interface {
	// Completed indicates that the current iteration of the reconciliation loop is complete, and the top-level
	// Reconcile() method should return [ReconcileResult.Output] to the controller runtime.
	//
	// This returns true for a [Done] or terminal [Error] (where the output will stop the entire reconciliation loop);
	// and for a [RequeueSoon] or regular [Error] (where the output will trigger a retry).
	//
	// This returns false for a [Continue].
	Completed() bool
	// Output converts this result into a format that the main Reconcile() method can return to the controller runtime.
	//
	// Calling this method on a [Continue] will panic.
	Output() (ctrl.Result, error)
	// IsError indicates whether this result is an [Error].
	IsError() bool
	// IsRequeue indicates whether this result is a [RequeueSoon].
	IsRequeue() bool
	// IsDone indicates whether this result is a [Done].
	IsDone() bool
	// GetError returns the wrapped error if the result is an [Error], otherwise it returns nil.
	GetError() error
}

type continueReconcile struct{}

func (c continueReconcile) Completed() bool {
	return false
}
func (c continueReconcile) Output() (ctrl.Result, error) {
	panic("there was no Result to return")
}

func (continueReconcile) IsDone() bool {
	return false
}

func (continueReconcile) IsError() bool {
	return false
}

func (continueReconcile) IsRequeue() bool {
	return false
}

func (continueReconcile) GetError() error {
	return nil
}

type done struct{}

func (d done) Completed() bool {
	return true
}
func (d done) Output() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (done) IsDone() bool {
	return true
}

func (done) IsError() bool {
	return false
}

func (done) IsRequeue() bool {
	return false
}

func (done) GetError() error {
	return nil
}

type callBackSoon struct {
	after time.Duration
}

func (c callBackSoon) Completed() bool {
	return true
}
func (c callBackSoon) Output() (ctrl.Result, error) {
	return ctrl.Result{Requeue: true, RequeueAfter: c.after}, nil
}

func (callBackSoon) IsDone() bool {
	return false
}

func (callBackSoon) IsError() bool {
	return false
}

func (callBackSoon) IsRequeue() bool {
	return true
}
func (callBackSoon) GetError() error {
	return nil
}

type errorOut struct {
	err error
}

func (e errorOut) Completed() bool {
	return true
}
func (e errorOut) Output() (ctrl.Result, error) {
	return ctrl.Result{}, e.err
}

func (errorOut) IsDone() bool {
	return false
}

func (errorOut) IsError() bool {
	return true
}

func (errorOut) IsRequeue() bool {
	return false
}

func (r errorOut) GetError() error {
	return r.err
}

// Continue indicates that the current step in the reconciliation is done. The caller should proceed with the next step.
func Continue() ReconcileResult {
	return continueReconcile{}
}

// Done indicates that the entire reconciliation loop was successful.
// The caller should skip the next steps (if any), and propagate the result up the stack. This will eventually reach the
// top-level Reconcile() method, which should stop the reconciliation.
func Done() ReconcileResult {
	return done{}
}

// RequeueSoon indicates that the current step in the reconciliation requires a requeue after a certain amount of time.
// The caller should skip the next steps (if any), and propagate the result up the stack. This will eventually reach the
// top-level Reconcile() method, which should schedule a requeue with the given delay.
func RequeueSoon(after time.Duration) ReconcileResult {
	return callBackSoon{after: after}
}

// Error indicates that the current step in the reconciliation has failed.
// The caller should skip the next steps (if any), and propagate the result up the stack. This will eventually reach the
// top-level Reconcile() method, which should return the error to the controller runtime.
//
// If the argument is wrapped with [reconcile.TerminalError], the reconciliation loop will stop; otherwise, it will be
// retried with exponential backoff.
func Error(e error) ReconcileResult {
	return errorOut{err: e}
}
