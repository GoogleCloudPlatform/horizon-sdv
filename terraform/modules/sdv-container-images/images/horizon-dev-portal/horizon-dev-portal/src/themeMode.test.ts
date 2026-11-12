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
import { afterEach, describe, expect, it, vi } from 'vitest';
import { readInitialDark, THEME_STORAGE_KEY } from './themeMode';

function stubLocalStorage(value: string | null | 'throw'): void {
  vi.stubGlobal('localStorage', {
    getItem: () => {
      if (value === 'throw') {
        throw new Error('storage disabled');
      }
      return value;
    },
  });
}

describe('readInitialDark', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses the stored theme preference first', () => {
    vi.stubGlobal('window', { APP_CONFIG: { theme: 'light' } });
    stubLocalStorage('dark');
    expect(readInitialDark()).toBe(true);

    stubLocalStorage('light');
    vi.stubGlobal('window', { APP_CONFIG: { theme: 'dark' } });
    expect(readInitialDark()).toBe(false);
  });

  it('falls back to APP_CONFIG when nothing is stored', () => {
    stubLocalStorage(null);
    vi.stubGlobal('window', { APP_CONFIG: { theme: 'dark' } });
    expect(readInitialDark()).toBe(true);

    vi.stubGlobal('window', { APP_CONFIG: {} });
    expect(readInitialDark()).toBe(false);
  });

  it('falls back to APP_CONFIG when storage throws', () => {
    stubLocalStorage('throw');
    vi.stubGlobal('window', { APP_CONFIG: { theme: 'dark' } });
    expect(readInitialDark()).toBe(true);
  });

  it('defaults to light when storage is unavailable and there is no window', () => {
    expect(readInitialDark()).toBe(false);
  });

  it('uses the theme storage key', () => {
    expect(THEME_STORAGE_KEY).toBe('theme');
  });
});
