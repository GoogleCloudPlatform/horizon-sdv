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
  clampDialogSize,
  readDialogLayoutCookie,
  writeDialogLayoutCookie,
} from './dialogLayoutCookie';

describe('dialog layout cookie', () => {
  let jar: string;
  let writes: string[];

  beforeEach(() => {
    jar = '';
    writes = [];
    vi.stubGlobal('document', {
      get cookie() {
        return jar;
      },
      set cookie(value: string) {
        writes.push(value);
      },
    });
    vi.stubGlobal('window', { APP_CONFIG: {} });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('reads a saved size from the cookie jar', () => {
    jar = `other=1; hdp_x=${encodeURIComponent(JSON.stringify({ width: 800, height: 600 }))}`;
    expect(readDialogLayoutCookie('hdp_x')).toEqual({ width: 800, height: 600 });
  });

  it('returns null for a missing, malformed, or non-numeric cookie', () => {
    expect(readDialogLayoutCookie('hdp_x')).toBeNull();

    jar = 'hdp_x=%7Bnot-json';
    expect(readDialogLayoutCookie('hdp_x')).toBeNull();

    jar = `hdp_x=${encodeURIComponent(JSON.stringify({ width: '800', height: 600 }))}`;
    expect(readDialogLayoutCookie('hdp_x')).toBeNull();
  });

  it('writes a one-year cookie scoped to the root path', () => {
    writeDialogLayoutCookie('hdp_x', 800, 600);
    expect(writes).toHaveLength(1);
    expect(writes[0]).toBe(
      'hdp_x=%7B%22width%22%3A800%2C%22height%22%3A600%7D;path=/;max-age=31536000;SameSite=Lax'
    );
  });

  it('ignores reads and writes when there is no document', () => {
    vi.unstubAllGlobals();
    expect(readDialogLayoutCookie('hdp_x')).toBeNull();
    expect(() => writeDialogLayoutCookie('hdp_x', 1, 1)).not.toThrow();
    expect(writes).toHaveLength(0);
  });
});

describe('clampDialogSize', () => {
  const viewport = { innerWidth: 1000, innerHeight: 800 };

  it('returns the size unchanged without a viewport', () => {
    expect(clampDialogSize(5000, 5000, 300, 200, null)).toEqual({ width: 5000, height: 5000 });
  });

  it('keeps a 16px gutter inside the viewport', () => {
    expect(clampDialogSize(5000, 5000, 300, 200, viewport)).toEqual({ width: 984, height: 784 });
  });

  it('never goes below the minimum size', () => {
    expect(clampDialogSize(10, 10, 300, 200, viewport)).toEqual({ width: 300, height: 200 });
  });

  it('shrinks the minimum when the viewport is smaller than the minimum', () => {
    const tiny = { innerWidth: 200, innerHeight: 100 };
    expect(clampDialogSize(10, 10, 300, 200, tiny)).toEqual({ width: 184, height: 84 });
  });

  it('keeps sizes that already fit', () => {
    expect(clampDialogSize(500, 400, 300, 200, viewport)).toEqual({ width: 500, height: 400 });
  });

  it('never returns less than 1px even for a zero-sized viewport', () => {
    expect(clampDialogSize(100, 100, 300, 200, { innerWidth: 0, innerHeight: 0 })).toEqual({
      width: 1,
      height: 1,
    });
  });
});
