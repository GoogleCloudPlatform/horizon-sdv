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
import { config } from './config';

describe('config.getString', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns the fallback when no window config exists', () => {
    expect(config.getString('keycloakUrl', 'fallback')).toBe('fallback');
  });

  it('returns a configured non-empty string', () => {
    vi.stubGlobal('window', { APP_CONFIG: { keycloakUrl: 'https://kc.example' } });
    expect(config.getString('keycloakUrl', 'fallback')).toBe('https://kc.example');
  });

  it('uses the fallback for empty or non-string values', () => {
    vi.stubGlobal('window', { APP_CONFIG: { keycloakUrl: '', horizonVersion: 42 } });
    expect(config.getString('keycloakUrl', 'fallback')).toBe('fallback');
    expect(config.getString('horizonVersion', 'fallback')).toBe('fallback');
  });
});
