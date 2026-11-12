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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  DEFAULT_MAX_WORKFLOW_LOG_LINES,
  MAX_MAX_WORKFLOW_LOG_LINES,
  MIN_MAX_WORKFLOW_LOG_LINES,
  readMaxWorkflowLogLines,
  writeMaxWorkflowLogLines,
} from './logBufferSettings';

const STORAGE_KEY = 'devportal.maxWorkflowLogLines';

describe('max workflow log lines setting', () => {
  let store: Map<string, string>;
  let dispatchEvent: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    store = new Map<string, string>();
    dispatchEvent = vi.fn();
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, v);
      },
    });
    vi.stubGlobal('window', {
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent,
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reads the default when nothing or junk is stored', () => {
    expect(readMaxWorkflowLogLines()).toBe(DEFAULT_MAX_WORKFLOW_LOG_LINES);
    store.set(STORAGE_KEY, 'abc');
    expect(readMaxWorkflowLogLines()).toBe(DEFAULT_MAX_WORKFLOW_LOG_LINES);
  });

  it('clamps stored values to the allowed range', () => {
    store.set(STORAGE_KEY, '1000');
    expect(readMaxWorkflowLogLines()).toBe(MIN_MAX_WORKFLOW_LOG_LINES);

    store.set(STORAGE_KEY, '999999');
    expect(readMaxWorkflowLogLines()).toBe(MAX_MAX_WORKFLOW_LOG_LINES);

    store.set(STORAGE_KEY, '60000');
    expect(readMaxWorkflowLogLines()).toBe(60_000);
  });

  it('rounds and clamps before storing, then notifies listeners', () => {
    expect(writeMaxWorkflowLogLines(8123.6)).toBe(8124);
    expect(store.get(STORAGE_KEY)).toBe('8124');
    expect(dispatchEvent).toHaveBeenCalledTimes(1);

    expect(writeMaxWorkflowLogLines(1)).toBe(MIN_MAX_WORKFLOW_LOG_LINES);
    expect(writeMaxWorkflowLogLines(1e9)).toBe(MAX_MAX_WORKFLOW_LOG_LINES);
  });

  it('returns the clamped value without touching storage when there is no window', () => {
    vi.unstubAllGlobals();
    expect(writeMaxWorkflowLogLines(10_000)).toBe(10_000);
    expect(readMaxWorkflowLogLines()).toBe(DEFAULT_MAX_WORKFLOW_LOG_LINES);
  });
});
