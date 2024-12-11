package medusa

import (
	"context"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RestoreRequest struct {
	Log logr.Logger

	RestoreJob *api.MedusaRestoreJob

	MedusaBackup *api.MedusaBackup

	Datacenter *cassdcapi.CassandraDatacenter

	restoreHash string

	datacenterHash string

	restorePatch client.Patch

	datacenterPatch client.Patch
}

type RequestFactory interface {
	// NewRestoreRequest Creates and initializes a RestoreRequest. The factory is
	// responsible for fetching the CassandraRestore, CassandraBackup, and
	// CassandraDatacenter objects from the api server. The factory returns a deep copy of
	// each of the objects, so there is no need to call DeepCopy() on them.
	// Reconciliation should proceed only if the returned Result is nil.
	NewMedusaRestoreRequest(ctx context.Context, restoreKey types.NamespacedName) (*RestoreRequest, *ctrl.Result, error)
}

type factory struct {
	client.Client

	Log logr.Logger
}

func NewFactory(client client.Client, logger logr.Logger) RequestFactory {
	return &factory{
		Client: client,
		Log:    logger,
	}
}

func (f *factory) NewMedusaRestoreRequest(ctx context.Context, restoreKey types.NamespacedName) (*RestoreRequest, *ctrl.Result, error) {
	restore := &api.MedusaRestoreJob{}
	err := f.Get(ctx, restoreKey, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, &ctrl.Result{}, nil
		}
		f.Log.Error(err, "Failed to get MedusaRestoreJob")
		return nil, &ctrl.Result{}, err
	}

	backup := &api.MedusaBackup{}
	backupKey := types.NamespacedName{Namespace: restoreKey.Namespace, Name: restore.Spec.Backup}
	err = f.Get(ctx, backupKey, backup)
	if err != nil {
		f.Log.Error(err, "Failed to get MedusaBackup", "MedusaBackup", backupKey)
		return nil, &ctrl.Result{}, err
	}

	dc := &cassdcapi.CassandraDatacenter{}
	dcKey := types.NamespacedName{Namespace: restoreKey.Namespace, Name: restore.Spec.CassandraDatacenter}
	err = f.Get(ctx, dcKey, dc)
	if err != nil {
		// TODO The datacenter does not have to exist for a remote restore
		f.Log.Error(err, "Failed to get CassandraDatacenter", "CassandraDatacenter", dcKey)
		return nil, &ctrl.Result{}, err
	}

	reqLogger := f.Log.WithValues(
		"MedusaRestoreJob", restoreKey,
		"MedusaBackup", backupKey,
		"CassandraDatacenter", dcKey)

	restoreHash := utils.DeepHashString(restore.Status)
	datacenterHash := utils.DeepHashString(dc.Spec)

	req := RestoreRequest{
		Log:             reqLogger,
		RestoreJob:      restore.DeepCopy(),
		MedusaBackup:    backup.DeepCopy(),
		Datacenter:      dc.DeepCopy(),
		restoreHash:     restoreHash,
		datacenterHash:  datacenterHash,
		restorePatch:    client.MergeFromWithOptions(restore.DeepCopy(), client.MergeFromWithOptimisticLock{}),
		datacenterPatch: client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{}),
	}

	return &req, nil, nil
}

// DatacenterModified returns true if the CassandraDatacenter.Spec has been modified.
func (r *RestoreRequest) DatacenterModified() bool {
	return utils.DeepHashString(r.Datacenter.Spec) != r.datacenterHash
}

// MedusaRestoreModified returns true if the CassandraRestore.Status has been modified.
func (r *RestoreRequest) MedusaRestoreModified() bool {
	return utils.DeepHashString(r.RestoreJob.Status) != r.restoreHash
}

// SetMedusaRestoreKey sets the key. Note that this function is idempotent.
func (r *RestoreRequest) SetMedusaRestoreKey(key string) {
	if len(r.RestoreJob.Status.RestoreKey) == 0 {
		r.RestoreJob.Status.RestoreKey = key
	}
}

// SetMedusaRestoreStartTime sets the start time. Note that this function is idempotent.
func (r *RestoreRequest) SetMedusaRestoreStartTime(t metav1.Time) {
	if r.RestoreJob.Status.StartTime.IsZero() {
		r.RestoreJob.Status.StartTime = t
	}
}

// SetDatacenterStoppedTimeRestoreJob sets the stop time.
func (r *RestoreRequest) SetDatacenterStoppedTimeRestoreJob(t metav1.Time) {
	if r.RestoreJob.Status.DatacenterStopped.IsZero() {
		r.RestoreJob.Status.DatacenterStopped = t
	}
}

func (r *RestoreRequest) SetMedusaRestoreFinishTime(time metav1.Time) {
	r.RestoreJob.Status.FinishTime = time
}

// SetRestorePrepared sets the prepared flag. Note that this function is idempotent.
func (r *RestoreRequest) SetMedusaRestorePrepared(prepared bool) {
	r.RestoreJob.Status.RestorePrepared = prepared
}

// SetRestorePrepared sets the prepared flag. Note that this function is idempotent.
func (r *RestoreRequest) SetMedusaRestoreMapping(restoreMapping api.MedusaRestoreMapping) {
	r.RestoreJob.Status.RestoreMapping = *restoreMapping.DeepCopy()
}

// GetRestorePatch returns a patch that can be used to apply changes to the CassandraRestore.
// The patch is created when the RestoreRequest is initialized.
func (r *RestoreRequest) GetRestorePatch() client.Patch {
	return r.restorePatch
}

// GetDatacenterPatch returns a patch that can be used to apply changes to the
// CassandraDatacenter. The patch is created when the RestoreRequest is initialized.
func (r *RestoreRequest) GetDatacenterPatch() client.Patch {
	return r.datacenterPatch
}
