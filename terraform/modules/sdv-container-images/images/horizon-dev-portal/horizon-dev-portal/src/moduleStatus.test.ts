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
//
import { describe, expect, it } from 'vitest';
import { deploymentStatus, isReady } from './moduleStatus';
import type { StatusResponse } from './types';

function status(partial: Partial<StatusResponse>): StatusResponse {
  return partial as StatusResponse;
}

describe('deploymentStatus', () => {
  it('reports UNINSTALL IN PROGRESS while an Application is being deleted', () => {
    expect(
      deploymentStatus(true, status({ applicationDeletionTimestamp: '2026-01-01T00:00:00Z' }))
    ).toBe('UNINSTALL IN PROGRESS');
  });

  it('reports INSTALLATION IN PROGRESS for an enabled prefixed module with missing children', () => {
    expect(
      deploymentStatus(
        true,
        status({ expectedManagedApplicationCount: 2, managedApplicationCount: 1 })
      )
    ).toBe('INSTALLATION IN PROGRESS');
    expect(
      deploymentStatus(
        true,
        status({
          expectedManagedApplicationCount: 2,
          managedApplicationCount: 2,
          managedChildApplicationPresent: false,
        })
      )
    ).toBe('INSTALLATION IN PROGRESS');
  });

  it('reports UNINSTALL IN PROGRESS for a parent-skip-reconcile prefixed module with missing children', () => {
    expect(
      deploymentStatus(
        true,
        status({
          expectedManagedApplicationCount: 2,
          managedApplicationCount: 0,
          parentSkipReconcile: true,
        })
      )
    ).toBe('UNINSTALL IN PROGRESS');
  });

  it('handles disabled modules by remaining Applications and Argo status', () => {
    expect(deploymentStatus(false, status({ remainingManagedApplications: 2 }))).toBe(
      'UNINSTALL IN PROGRESS'
    );
    expect(deploymentStatus(false, status({ remainingManagedApplications: 0 }))).toBe(
      'NOT INSTALLED'
    );
    expect(deploymentStatus(false, status({ syncStatus: 'Synced' }))).toBe('UNINSTALL IN PROGRESS');
    expect(deploymentStatus(false, undefined)).toBe('NOT INSTALLED');
  });

  it('reports DEPLOYMENT PENDING when enabled with no Applications and no Argo status', () => {
    expect(deploymentStatus(true, status({ managedApplicationCount: 0 }))).toBe(
      'DEPLOYMENT PENDING'
    );
  });

  it('reports READY when synced and healthy with at least one Application', () => {
    expect(
      deploymentStatus(
        true,
        status({ syncStatus: 'Synced', healthStatus: 'Healthy', managedApplicationCount: 1 })
      )
    ).toBe('READY');
  });

  it('reports INSTALLATION IN PROGRESS when synced and healthy but with zero Applications', () => {
    expect(
      deploymentStatus(
        true,
        status({ syncStatus: 'Synced', healthStatus: 'Healthy', managedApplicationCount: 0 })
      )
    ).toBe('INSTALLATION IN PROGRESS');
  });

  it('reports UPDATE IN PROGRESS for drift on a healthy deploy or an operation in flight', () => {
    expect(
      deploymentStatus(true, status({ syncStatus: 'OutOfSync', healthStatus: 'Healthy' }))
    ).toBe('UPDATE IN PROGRESS');
    expect(
      deploymentStatus(
        true,
        status({
          syncStatus: 'Synced',
          healthStatus: 'Healthy',
          operationPhase: 'Running',
          managedApplicationCount: 1,
        })
      )
    ).toBe('UPDATE IN PROGRESS');
  });

  it('falls back to INSTALLATION IN PROGRESS while the first install is progressing', () => {
    expect(
      deploymentStatus(true, status({ syncStatus: 'Unknown', healthStatus: 'Progressing' }))
    ).toBe('INSTALLATION IN PROGRESS');
  });
});

describe('isReady', () => {
  it('is true only for READY', () => {
    expect(
      isReady(
        true,
        status({ syncStatus: 'Synced', healthStatus: 'Healthy', managedApplicationCount: 1 })
      )
    ).toBe(true);
    expect(isReady(true, status({ managedApplicationCount: 0 }))).toBe(false);
  });
});
