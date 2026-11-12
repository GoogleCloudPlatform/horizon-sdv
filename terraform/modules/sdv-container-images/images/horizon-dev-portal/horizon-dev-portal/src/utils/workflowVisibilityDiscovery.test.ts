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
import type { WorkflowSummary } from '../types';
import {
  collectSubmittedFromFromWorkflows,
  extractSubmittedFromEnumsFromOpenAPI,
  mergeSourceOptions,
  workflowPassesSubmittedFromFilter,
} from './workflowVisibilityDiscovery';

const CANONICAL = ['api', 'developer-portal', 'horizon-cli'];

function wf(submittedFrom?: string): WorkflowSummary {
  return { name: 'wf', namespace: 'ns', phase: 'Running', submittedFrom } as WorkflowSummary;
}

describe('extractSubmittedFromEnumsFromOpenAPI', () => {
  it('collects header and property enums and always includes canonical sources', () => {
    const doc = {
      paths: {
        '/x': {
          get: {
            parameters: [
              {
                name: 'X-Horizon-Submitted-From',
                schema: { enum: [' custom ', 'api', ''] },
              },
            ],
          },
        },
      },
      components: {
        schemas: {
          Workflow: { properties: { submittedFrom: { enum: ['zeta'] } } },
        },
      },
    };
    expect(extractSubmittedFromEnumsFromOpenAPI(doc)).toEqual([
      'api',
      'custom',
      'developer-portal',
      'horizon-cli',
      'zeta',
    ]);
  });

  it('returns only the canonical sources for an empty document', () => {
    expect(extractSubmittedFromEnumsFromOpenAPI(null)).toEqual(CANONICAL);
  });
});

describe('collectSubmittedFromFromWorkflows', () => {
  it('trims, drops blanks, de-duplicates, and sorts', () => {
    const items = [wf(' b '), wf('a'), wf('  '), wf(undefined), wf('b')];
    expect(collectSubmittedFromFromWorkflows(items)).toEqual(['a', 'b']);
  });
});

describe('mergeSourceOptions', () => {
  it('unions both lists and sorts the result', () => {
    expect(mergeSourceOptions(['b', 'a'], ['a', 'c'])).toEqual(['a', 'b', 'c']);
  });
});

describe('workflowPassesSubmittedFromFilter', () => {
  it('shows everything when no restriction is set', () => {
    expect(workflowPassesSubmittedFromFilter(wf('api'), null)).toBe(true);
    expect(workflowPassesSubmittedFromFilter(wf('api'), undefined)).toBe(true);
  });

  it('hides everything for an empty allow-list', () => {
    expect(workflowPassesSubmittedFromFilter(wf('api'), [])).toBe(false);
  });

  it('matches the trimmed submittedFrom against the allow-list', () => {
    expect(workflowPassesSubmittedFromFilter(wf('api'), ['api'])).toBe(true);
    expect(workflowPassesSubmittedFromFilter(wf('api'), ['horizon-cli'])).toBe(false);
    expect(workflowPassesSubmittedFromFilter(wf(undefined), ['api'])).toBe(false);
  });
});
