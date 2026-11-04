// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
	gcsv1alpha1 "storage-gcs-module/api/v1alpha1"
	"storage-gcs-module/internal/gcsname"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const gcsBucketFinalizer = "gcsmanager.horizon.io/gcsbucket-finalizer"

// GCSBucketReconciler reconciles GCSBucket CRs against Cloud Storage.
type GCSBucketReconciler struct {
	client.Client
	GCS       *storage.Client
	ProjectID string
}

// +kubebuilder:rbac:groups=gcsmanager.horizon.io,resources=gcsbuckets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gcsmanager.horizon.io,resources=gcsbuckets/status,verbs=get;update;patch

func uniformUBLASpec(spec gcsv1alpha1.GCSBucketSpec) bool {
	if spec.UniformBucketLevelAccess == nil {
		return true
	}
	return *spec.UniformBucketLevelAccess
}

func (r *GCSBucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj gcsv1alpha1.GCSBucket
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if obj.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, &obj)
	}

	// Finalizer ensures the operator empties and deletes the GCP bucket before the CR is removed
	// (e.g. when the owning module is disabled in the Developer Portal).
	if !controllerutil.ContainsFinalizer(&obj, gcsBucketFinalizer) {
		controllerutil.AddFinalizer(&obj, gcsBucketFinalizer)
		if err := r.Update(ctx, &obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileCreateUpdate(ctx, &obj)
}

func (r *GCSBucketReconciler) reconcileCreateUpdate(ctx context.Context, obj *gcsv1alpha1.GCSBucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if r.ProjectID == "" {
		obj.Status = gcsv1alpha1.GCSBucketStatus{
			Phase:   "Failed",
			Message: "GOOGLE_CLOUD_PROJECT is not set on the operator",
		}
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	spec := obj.Spec
	if spec.BucketName == "" || spec.Location == "" {
		obj.Status = gcsv1alpha1.GCSBucketStatus{
			Phase:   "Failed",
			Message: "spec.bucketName and spec.location are required",
		}
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if !gcsname.HasProjectPrefix(r.ProjectID, spec.BucketName) {
		obj.Status = gcsv1alpha1.GCSBucketStatus{
			Phase:   "Failed",
			Message: fmt.Sprintf("spec.bucketName %q must start with %q", spec.BucketName, r.ProjectID+"-"),
		}
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	bh := r.GCS.Bucket(spec.BucketName)
	attrs, err := bh.Attrs(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrBucketNotExist) {
			logger.Error(err, "bucket Attrs")
			obj.Status = gcsv1alpha1.GCSBucketStatus{
				Phase:   "Failed",
				Message: err.Error(),
			}
			if err := r.Status().Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		ubla := uniformUBLASpec(spec)
		battrs := &storage.BucketAttrs{
			Location: spec.Location,
			UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
				Enabled: ubla,
			},
			Labels: spec.Labels,
		}
		if err := bh.Create(ctx, r.ProjectID, battrs); err != nil {
			logger.Error(err, "bucket Create")
			obj.Status = gcsv1alpha1.GCSBucketStatus{
				Phase:   "Failed",
				Message: err.Error(),
			}
			if err := r.Status().Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		obj.Status = gcsv1alpha1.GCSBucketStatus{
			Phase:   "Ready",
			Message: "Bucket created",
			GsURI:   "gs://" + spec.BucketName,
		}
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Bucket already exists: location must match (GCS does not allow changing location).
	// GCS may return canonical casing (e.g. US-CENTRAL1) while spec uses API-style (us-central1).
	if !strings.EqualFold(strings.TrimSpace(attrs.Location), strings.TrimSpace(spec.Location)) {
		obj.Status = gcsv1alpha1.GCSBucketStatus{
			Phase:   "Failed",
			Message: fmt.Sprintf("bucket exists in location %q but spec.location is %q", attrs.Location, spec.Location),
			GsURI:   "gs://" + spec.BucketName,
		}
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	obj.Status = gcsv1alpha1.GCSBucketStatus{
		Phase:   "Ready",
		Message: "Bucket exists",
		GsURI:   "gs://" + spec.BucketName,
	}
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileDelete removes the CR. Retain (default) keeps the GCP bucket.
// Delete empties all object generations and deletes the GCP bucket.
func (r *GCSBucketReconciler) reconcileDelete(ctx context.Context, obj *gcsv1alpha1.GCSBucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("bucket", obj.Spec.BucketName)

	if !obj.Spec.DeleteGCPBucketOnCRDelete() {
		logger.Info("retaining GCP bucket (spec.deletionPolicy=Retain)")
		return r.clearFinalizer(ctx, obj)
	}

	bh := r.GCS.Bucket(obj.Spec.BucketName)

	if _, err := bh.Attrs(ctx); err != nil {
		if errors.Is(err, storage.ErrBucketNotExist) {
			return r.clearFinalizer(ctx, obj)
		}
		logger.Error(err, "bucket Attrs during delete")
		return ctrl.Result{}, err
	}

	if err := bh.Delete(ctx); err != nil {
		if !isBucketNotEmpty(err) {
			logger.Error(err, "bucket Delete")
			return ctrl.Result{}, err
		}
		logger.Info("emptying GCS bucket before delete")
		if err := emptyBucket(ctx, bh); err != nil {
			logger.Error(err, "empty bucket")
			return ctrl.Result{}, err
		}
		if err := bh.Delete(ctx); err != nil {
			logger.Error(err, "bucket Delete after empty")
			return ctrl.Result{}, err
		}
	}

	logger.Info("GCS bucket deleted")
	return r.clearFinalizer(ctx, obj)
}

func (r *GCSBucketReconciler) clearFinalizer(ctx context.Context, obj *gcsv1alpha1.GCSBucket) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, gcsBucketFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(obj, gcsBucketFinalizer)
	if err := r.Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// emptyBucket deletes all object generations so the bucket can be removed.
func emptyBucket(ctx context.Context, bh *storage.BucketHandle) error {
	it := bh.Objects(ctx, &storage.Query{Versions: true})
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return nil
		}
		if err != nil {
			return err
		}
		obj := bh.Object(attrs.Name)
		if attrs.Generation != 0 {
			obj = obj.Generation(attrs.Generation)
		}
		if err := obj.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return err
		}
	}
}

func isBucketNotEmpty(err error) bool {
	var ge *googleapi.Error
	if errors.As(err, &ge) && ge.Code == 409 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not empty") ||
		strings.Contains(strings.ToLower(err.Error()), "must be empty")
}

// SetupWithManager wires the reconciler.
func (r *GCSBucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gcsv1alpha1.GCSBucket{}).
		Complete(r)
}