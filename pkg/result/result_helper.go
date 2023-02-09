package result

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// Copyright DataStax, Inc.
// Please see the included license file for details.

type ReconcileResult interface {
	Completed() bool
	Output() (ctrl.Result, error)
	IsError() bool
	IsRequeue() bool
	IsDone() bool
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
