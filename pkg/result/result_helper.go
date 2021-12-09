package result

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// Copyright DataStax, Inc.
// Please see the included license file for details.

type ReconcileResult interface {
	Completed() bool
	Output() (ctrl.Result, error)
}

type continueReconcile struct{}

func (c continueReconcile) Completed() bool {
	return false
}
func (c continueReconcile) Output() (ctrl.Result, error) {
	panic("there was no Result to return")
}

type done struct{}

func (d done) Completed() bool {
	return true
}
func (d done) Output() (ctrl.Result, error) {
	return ctrl.Result{}, nil
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

type errorOut struct {
	err error
}

func (e errorOut) Completed() bool {
	return true
}
func (e errorOut) Output() (ctrl.Result, error) {
	return ctrl.Result{}, e.err
}

func Continue() ReconcileResult {
	return continueReconcile{}
}

func Done() ReconcileResult {
	return done{}
}

func RequeueSoon(after time.Duration) ReconcileResult {
	return callBackSoon{after: after}
}

func Error(e error) ReconcileResult {
	return errorOut{err: e}
}
