package medusa

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/hash"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RestoreRequest struct {
	Log logr.Logger

	Restore *api.CassandraRestore

	Backup *api.CassandraBackup

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
	NewRestoreRequest(ctx context.Context, restoreKey types.NamespacedName) (*RestoreRequest, *ctrl.Result, error)
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

func (f *factory) NewRestoreRequest(ctx context.Context, restoreKey types.NamespacedName) (*RestoreRequest, *ctrl.Result, error) {
	restore := &api.CassandraRestore{}
	err := f.Get(ctx, restoreKey, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, &ctrl.Result{}, nil
		}
		f.Log.Error(err, "Failed to get CassandraRestore")
		return nil, &ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	backup := &api.CassandraBackup{}
	backupKey := types.NamespacedName{Namespace: restoreKey.Namespace, Name: restore.Spec.Backup}
	err = f.Get(ctx, backupKey, backup)
	if err != nil {
		f.Log.Error(err, "Failed to get CassandraBackup", "CassandraBackup", backupKey)
		return nil, &ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	dc := &cassdcapi.CassandraDatacenter{}
	dcKey := types.NamespacedName{Namespace: restoreKey.Namespace, Name: restore.Spec.CassandraDatacenter.Name}
	err = f.Get(ctx, dcKey, dc)
	if err != nil {
		// TODO The datacenter does not have to exist for a remote restore
		f.Log.Error(err, "Failed to get CassandraDatacenter", "CassandraDatacenter", dcKey)
		return nil, &ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	reqLogger := f.Log.WithValues(
		"CassandraRestore", restoreKey,
		"CassandraBackup", backupKey,
		"CassandraDatacenter", dcKey)

	restoreHash := deepHashString(restore.Status)
	datacenterHash := deepHashString(dc.Spec)

	req := RestoreRequest{
		Log:             reqLogger,
		Restore:         restore.DeepCopy(),
		Backup:          backup.DeepCopy(),
		Datacenter:      dc.DeepCopy(),
		restoreHash:     restoreHash,
		datacenterHash:  datacenterHash,
		restorePatch:    client.MergeFromWithOptions(restore.DeepCopy(), client.MergeFromWithOptimisticLock{}),
		datacenterPatch: client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{}),
	}

	return &req, nil, nil
}

// RestoreModified returns true if the CassandraRestore.Status has been modified.
func (r *RestoreRequest) RestoreModified() bool {
	return deepHashString(r.Restore.Status) != r.restoreHash
}

// DatacenterModified returns true if the CassandraDatacenter.Spec has been modified.
func (r *RestoreRequest) DatacenterModified() bool {
	return deepHashString(r.Datacenter.Spec) != r.datacenterHash
}

// SetRestoreKey sets the key. Note that this function is idempotent.
func (r *RestoreRequest) SetRestoreKey(key string) {
	if len(r.Restore.Status.RestoreKey) == 0 {
		r.Restore.Status.RestoreKey = key
	}
}

// SetRestoreStartTime sets the start time. Note that this function is idempotent.
func (r *RestoreRequest) SetRestoreStartTime(t metav1.Time) {
	if r.Restore.Status.StartTime.IsZero() {
		r.Restore.Status.StartTime = t
	}
}

// SetDatacenterStoppedTime sets the stop time.
func (r *RestoreRequest) SetDatacenterStoppedTime(t metav1.Time) {
	if r.Restore.Status.DatacenterStopped.IsZero() {
		r.Restore.Status.DatacenterStopped = t
	}
}

func (r *RestoreRequest) SetRestoreFinishTime(time metav1.Time) {
	r.Restore.Status.FinishTime = time
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

func deepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}
