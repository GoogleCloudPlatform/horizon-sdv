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
import { deploymentStatus, isReady } from './moduleStatus';
import type { ModuleResponse, StatusResponse } from './types';
import { apiMm } from './utils/api';

/** Per-module status can be slow (large Argo apps); do not block other modules or hang the sidebar tick forever. */
const moduleStatusFetchTimeoutMs = 25_000;

/** Sidebar lists modules that are READY for use, or still tearing down (UNINSTALL IN PROGRESS) so the entry does not vanish while Argo prunes. */
export async function fetchNavModuleNameIfRelevant(m: ModuleResponse): Promise<string | null> {
  const ctrl = new AbortController();
  const tid = window.setTimeout(() => ctrl.abort(), moduleStatusFetchTimeoutMs);
  try {
    const sr = await apiMm(`/modules/${encodeURIComponent(m.name)}/status`, {
      signal: ctrl.signal,
    });
    if (!sr.ok) {
      return null;
    }
    const st = (await sr.json()) as StatusResponse;
    const label = deploymentStatus(m.enabled, st);
    if (isReady(m.enabled, st) || label === 'UNINSTALL IN PROGRESS') {
      return m.name;
    }
    return null;
  } catch {
    return null;
  } finally {
    window.clearTimeout(tid);
  }
}

export async function fetchNavModuleNames(): Promise<string[]> {
  const r = await apiMm('/modules');
  if (!r.ok) {
    return [];
  }
  const list = (await r.json()) as ModuleResponse[];
  const parts = await Promise.all(list.map((m) => fetchNavModuleNameIfRelevant(m)));
  const names = parts.filter((x): x is string => x != null);
  return names.sort((a, b) => a.localeCompare(b));
}
