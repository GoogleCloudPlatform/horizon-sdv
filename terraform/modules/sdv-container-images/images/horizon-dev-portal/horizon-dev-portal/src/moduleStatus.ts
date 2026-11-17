// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
//
import type { StatusResponse } from './types';

export type DeploymentStatus =
  | 'NOT INSTALLED'
  | 'UNINSTALL IN PROGRESS'
  | 'DEPLOYMENT PENDING'
  | 'INSTALLATION IN PROGRESS'
  | 'UPDATE IN PROGRESS'
  | 'READY';

/** workloads-android / workloads-common: parent-only is not a full deploy (enable or disable in flight). */
function prefixedChildStackIncomplete(st?: StatusResponse): boolean {
  if (st?.expectedManagedApplicationCount !== 2) {
    return false;
  }
  const mc = st.managedApplicationCount;
  if (typeof mc === 'number' && mc < 2) {
    return true;
  }
  if (st.managedChildApplicationPresent === false) {
    return true;
  }
  return false;
}

function argoAppStatusPresent(st?: StatusResponse): boolean {
  if (!st) {
    return false;
  }
  return !!(
    (st.syncStatus ?? '').trim() ||
    (st.healthStatus ?? '').trim() ||
    (st.operationPhase ?? '').trim() ||
    (st.desiredRevision ?? '').trim() ||
    (st.syncRevision ?? '').trim() ||
    (st.applicationDeletionTimestamp ?? '').trim()
  );
}

/**
 * Maps Argo CD Application status to a coarse portal label.
 * UPDATE IN PROGRESS: drift on a previously healthy deploy (OutOfSync+Healthy), or an
 * operation in flight while the app was already synced/healthy (typical in-place update).
 * First-time install usually has health Progressing or sync Unknown, so it stays INSTALLATION.
 *
 * Disable / uninstall: prefer `applicationDeletionTimestamp` (child or parent terminating) or
 * `enabled === false` with remaining Argo apps — those map to UNINSTALL IN PROGRESS. If the UI
 * still showed INSTALLATION/UPDATE during disable, it was usually because ModuleManagerState had
 * not flipped yet (child teardown) or the admin tab skipped background poll during `busy` for
 * enable only—disable keeps polling so UNINSTALL appears while DELETE is in flight. Server-side
 * ordering (state before parent delete) and CIT ordering after child Application removal reduce
 * misleading READY during teardown. Prefixed-child modules (expectedManagedApplicationCount === 2):
 * parent-only maps to INSTALLATION IN PROGRESS (enable) or UNINSTALL IN PROGRESS (parent skip-reconcile).
 * Other modules (cloud-workstations drain): enabled with a paused parent maps to UNINSTALL IN PROGRESS.
 *
 * DEPLOYMENT PENDING: module is enabled, the API reports zero managed Argo CD Applications, and
 * there is no Argo-derived status yet — GitOps has not materialized the Application(s), or the
 * parent name/namespace does not match the cluster. This is distinct from INSTALLATION IN PROGRESS,
 * which implies an Application exists and is still syncing or becoming healthy.
 */
export function deploymentStatus(enabled: boolean, st?: StatusResponse): DeploymentStatus {
  // Parent or child Application deleting while GET /modules may still show enabled (disable ordering).
  if ((st?.applicationDeletionTimestamp ?? '').trim()) {
    return 'UNINSTALL IN PROGRESS';
  }
  if (enabled && prefixedChildStackIncomplete(st)) {
    return st?.parentSkipReconcile ? 'UNINSTALL IN PROGRESS' : 'INSTALLATION IN PROGRESS';
  }
  if (enabled && st?.parentSkipReconcile && st?.expectedManagedApplicationCount !== 2) {
    return 'UNINSTALL IN PROGRESS';
  }
  if (!enabled) {
    const rem = st?.remainingManagedApplications;
    if (rem != null && rem > 0) {
      return 'UNINSTALL IN PROGRESS';
    }
    if (rem === 0 && !argoAppStatusPresent(st)) {
      return 'NOT INSTALLED';
    }
    return argoAppStatusPresent(st) ? 'UNINSTALL IN PROGRESS' : 'NOT INSTALLED';
  }
  const mc = st?.managedApplicationCount;
  if (typeof mc === 'number' && mc === 0 && !argoAppStatusPresent(st)) {
    return 'DEPLOYMENT PENDING';
  }
  const sync = (st?.syncStatus ?? '').trim();
  const health = (st?.healthStatus ?? '').trim();
  const op = (st?.operationPhase ?? '').trim();
  const opBusy = op === 'Running' || op === 'Pending';

  if (sync === 'Synced' && health === 'Healthy' && !opBusy) {
    if (typeof mc === 'number' && mc === 0) {
      return 'INSTALLATION IN PROGRESS';
    }
    return 'READY';
  }
  if (sync === 'OutOfSync' && health === 'Healthy') {
    return 'UPDATE IN PROGRESS';
  }
  if (opBusy && (health === 'Healthy' || sync === 'Synced')) {
    return 'UPDATE IN PROGRESS';
  }
  return 'INSTALLATION IN PROGRESS';
}

export function isReady(enabled: boolean, st?: StatusResponse): boolean {
  return deploymentStatus(enabled, st) === 'READY';
}
