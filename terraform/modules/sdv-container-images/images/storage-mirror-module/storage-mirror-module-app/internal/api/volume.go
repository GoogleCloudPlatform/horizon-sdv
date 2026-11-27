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

package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/catalog"
	"storage-mirror-module/internal/controller"
	"storage-mirror-module/internal/state"
	"storage-mirror-module/internal/volume"
)

func (s *Server) getVolume(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"instance":     volume.InstanceID(cfg.Spec.Volume),
		"tier":         cfg.Spec.Volume.Tier,
		"zone":         cfg.Spec.Volume.Zone,
		"capacityGb":   cfg.Spec.Volume.CapacityGb,
		"allowDestroy": cfg.Spec.Volume.AllowDestroy,
		"status":       cfg.Status.Volume,
		"conditions":   cfg.Status.Conditions,
	})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.config(r.Context())
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"spec": cfg.Spec, "status": cfg.Status})
}

// destroyVolume is the API half of the two-key destroy: Git must set spec.volume.allowDestroy.
func (s *Server) destroyVolume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg, err := s.config(ctx)
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	instance := volume.InstanceID(cfg.Spec.Volume)
	if !cfg.Spec.Volume.AllowDestroy {
		writeErr(w, http.StatusForbidden, "volume destroy needs spec.volume.allowDestroy=true in Git")
		return
	}
	if r.URL.Query().Get("confirm") != instance {
		writeErr(w, http.StatusBadRequest, "pass confirm=<instance name> ("+instance+")")
		return
	}
	var list smv1.StorageMirrorList
	if err := s.live().List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(list.Items) > 0 && r.URL.Query().Get("force") != "true" {
		writeErr(w, http.StatusConflict, "mirrors still exist; delete them first or pass force=true")
		return
	}
	patch := client.MergeFrom(cfg.DeepCopy())
	if cfg.Annotations == nil {
		cfg.Annotations = map[string]string{}
	}
	cfg.Annotations[controller.AnnotationVolumeDestroyed] = "true"
	if err := s.Client.Patch(ctx, cfg, patch); err != nil {
		writeK8sErr(w, err)
		return
	}
	for _, c := range volume.Consumers(cfg.Spec) {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := s.live().Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: c.ClaimName}, pvc); err == nil && pvc.Labels[smv1.LabelModule] == smv1.ModuleName {
			_ = s.Client.Delete(ctx, pvc)
		}
		pv := &corev1.PersistentVolume{}
		if err := s.live().Get(ctx, client.ObjectKey{Name: c.PVName}, pv); err == nil && pv.Labels[smv1.LabelModule] == smv1.ModuleName {
			_ = s.Client.Delete(ctx, pv)
		}
	}
	fs := &unstructured.Unstructured{}
	fs.SetGroupVersionKind(volume.FilestoreGVK)
	if err := s.live().Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: volume.FilestoreInstanceName}, fs); err == nil {
		ann := fs.GetAnnotations()
		ann["cnrm.cloud.google.com/deletion-policy"] = "delete"
		fs.SetAnnotations(ann)
		if err := s.Client.Update(ctx, fs); err != nil {
			writeK8sErr(w, err)
			return
		}
		if err := s.Client.Delete(ctx, fs); err != nil && !apierrors.IsNotFound(err) {
			writeK8sErr(w, err)
			return
		}
	}
	// The mirrors and the catalog lived on the destroyed share: release the CRs without a delete run (Retain) and
	// drop the snapshot, so a recreated volume starts from volume-init.
	for i := range list.Items {
		m := &list.Items[i]
		patch := client.MergeFrom(m.DeepCopy())
		m.Spec.DeletionPolicy = smv1.DeletionPolicyRetain
		if m.Annotations == nil {
			m.Annotations = map[string]string{}
		}
		m.Annotations[smv1.AnnotationDeleteRequestedBy] = user(r)
		if err := s.Client.Patch(ctx, m, patch); client.IgnoreNotFound(err) != nil {
			writeK8sErr(w, err)
			return
		}
		if err := s.Client.Delete(ctx, m); client.IgnoreNotFound(err) != nil {
			writeK8sErr(w, err)
			return
		}
	}
	cm := &corev1.ConfigMap{}
	cm.Name, cm.Namespace = catalog.ConfigMapName, s.Namespace
	if err := s.Client.Delete(ctx, cm); client.IgnoreNotFound(err) != nil {
		writeK8sErr(w, err)
		return
	}
	log.Printf("storage-mirror volume %s destroyed by %q (%d mirrors released)", instance, user(r), len(list.Items))
	s.wakeConfig()
	writeJSON(w, http.StatusAccepted, map[string]string{"instance": instance, "status": "destroying"})
}

// ProgressReport is the PUT /internal/v1/runs/{runId}/progress body.
type ProgressReport struct {
	Mirror      string        `json:"mirror"`
	Progress    smv1.Progress `json:"progress"`
	LFSDetected bool          `json:"lfsDetected,omitempty"`
}

func (s *Server) putProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	run := r.PathValue("runId")
	var rep ProgressReport
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&rep); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	wf := workflowObject()
	if err := s.reader().Get(ctx, client.ObjectKey{Namespace: s.WorkflowNamespace, Name: run}, wf); err != nil {
		writeErr(w, http.StatusNotFound, "unknown run")
		return
	}
	labels := wf.GetLabels()
	if labels[smv1.LabelModule] != smv1.ModuleName || labels[smv1.LabelMirror] != rep.Mirror {
		writeErr(w, http.StatusForbidden, "run does not belong to this mirror")
		return
	}
	rep.Progress.UpdatedAt = nil
	s.Progress.Put(run, state.Report{Mirror: rep.Mirror, Progress: rep.Progress, LFSDetected: rep.LFSDetected, ReceivedAt: s.now()})
	s.wakeMirror(rep.Mirror)
	w.WriteHeader(http.StatusNoContent)
}

// CatalogReport is the POST /internal/v1/catalog body.
type CatalogReport struct {
	MetadataYAML string           `json:"metadataYaml"`
	Usage        controller.Usage `json:"usage"`
}

func (s *Server) postCatalog(w http.ResponseWriter, r *http.Request) {
	var rep CatalogReport
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCatalogBody)).Decode(&rep); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	root := "horizon-mirrors"
	if cfg, err := s.config(r.Context()); err == nil {
		root = controller.RootKey(cfg)
	}
	if _, err := catalog.Parse([]byte(rep.MetadataYAML), root); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	usage, _ := json.Marshal(rep.Usage)
	if err := catalog.Store(r.Context(), s.Client, s.Namespace, []byte(rep.MetadataYAML), usage, s.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.wakeConfig()
	w.WriteHeader(http.StatusNoContent)
}
