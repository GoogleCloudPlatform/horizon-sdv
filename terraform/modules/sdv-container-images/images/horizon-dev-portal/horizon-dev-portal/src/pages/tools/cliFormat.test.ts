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
import {
  artifactKey,
  cliApiPath,
  detectPlatform,
  filenameFromDisposition,
  formatBuildDate,
  formatBytes,
  linuxMacCliFilename,
  preferredCliArtifact,
} from './cliFormat';

describe('formatBytes', () => {
  it('shows a dash for invalid or negative sizes', () => {
    expect(formatBytes(Number.NaN)).toBe('—');
    expect(formatBytes(-1)).toBe('—');
  });

  it('picks B, KiB, or MiB by magnitude', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(2048)).toBe('2.0 KiB');
    expect(formatBytes(1536)).toBe('1.5 KiB');
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MiB');
  });
});

describe('formatBuildDate', () => {
  it('returns unknown for a missing or invalid date', () => {
    expect(formatBuildDate(undefined)).toBe('unknown');
    expect(formatBuildDate('')).toBe('unknown');
    expect(formatBuildDate('not a date')).toBe('unknown');
  });

  it('formats a valid date as dd/mm/yyyy', () => {
    // Local noon keeps the calendar day stable in every time zone.
    expect(formatBuildDate(new Date(2026, 2, 5, 12).toISOString())).toBe('05/03/2026');
  });
});

describe('filenameFromDisposition', () => {
  it('falls back when the header is missing or has no filename', () => {
    expect(filenameFromDisposition(null, 'fallback')).toBe('fallback');
    expect(filenameFromDisposition('attachment', 'fallback')).toBe('fallback');
  });

  it('prefers the RFC 5987 filename* parameter and decodes it', () => {
    expect(
      filenameFromDisposition("attachment; filename*=UTF-8''horizon%20cli.exe", 'fallback')
    ).toBe('horizon cli.exe');
  });

  it('keeps the raw value when filename* cannot be decoded', () => {
    expect(filenameFromDisposition("attachment; filename*=UTF-8''bad%ZZ", 'fallback')).toBe(
      'bad%ZZ'
    );
  });

  it('reads quoted and plain filename parameters', () => {
    expect(filenameFromDisposition('attachment; filename="horizon.exe"', 'fallback')).toBe(
      'horizon.exe'
    );
    expect(filenameFromDisposition('attachment; filename=horizon; size=1', 'fallback')).toBe(
      'horizon'
    );
  });
});

describe('detectPlatform', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('detects Windows on amd64 from the platform string', async () => {
    vi.stubGlobal('navigator', { platform: 'Win32', userAgent: 'Mozilla/5.0 (Windows NT 10.0)' });
    await expect(detectPlatform()).resolves.toEqual({ os: 'windows', arch: 'amd64' });
  });

  it('prefers high-entropy client hints for macOS on Apple Silicon', async () => {
    vi.stubGlobal('navigator', {
      platform: 'MacIntel',
      userAgent: 'Mozilla/5.0 (Macintosh)',
      userAgentData: {
        platform: 'macOS',
        architecture: 'x86',
        getHighEntropyValues: async () => ({ platform: 'macOS', architecture: 'arm' }),
      },
    });
    await expect(detectPlatform()).resolves.toEqual({ os: 'darwin', arch: 'arm64' });
  });

  it('falls back to the low-entropy hints when high-entropy values fail', async () => {
    vi.stubGlobal('navigator', {
      platform: '',
      userAgent: 'Mozilla/5.0',
      userAgentData: {
        platform: 'Windows',
        getHighEntropyValues: async () => {
          throw new Error('not allowed');
        },
      },
    });
    await expect(detectPlatform()).resolves.toEqual({ os: 'windows', arch: 'amd64' });
  });

  it('defaults to Linux and detects aarch64 from the user agent', async () => {
    vi.stubGlobal('navigator', {
      platform: 'Linux x86_64',
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64)',
    });
    await expect(detectPlatform()).resolves.toEqual({ os: 'linux', arch: 'amd64' });

    vi.stubGlobal('navigator', {
      platform: 'Linux aarch64',
      userAgent: 'Mozilla/5.0 (X11; Linux aarch64)',
    });
    await expect(detectPlatform()).resolves.toEqual({ os: 'linux', arch: 'arm64' });
  });
});

describe('artifactKey', () => {
  it('joins os and arch', () => {
    expect(artifactKey({ os: 'linux', arch: 'arm64' })).toBe('linux/arm64');
  });
});

describe('preferredCliArtifact', () => {
  const artifacts = [
    { os: 'linux', arch: 'amd64', id: 'linux' },
    { os: 'darwin', arch: 'arm64', id: 'mac-arm' },
  ];

  it('prefers an exact os and arch match', () => {
    expect(preferredCliArtifact(artifacts, { os: 'darwin', arch: 'arm64' })?.id).toBe('mac-arm');
  });

  it('falls back to any artifact for the same os', () => {
    expect(preferredCliArtifact(artifacts, { os: 'darwin', arch: 'amd64' })?.id).toBe('mac-arm');
  });

  it('returns null when nothing matches or inputs are missing', () => {
    expect(preferredCliArtifact(artifacts, { os: 'windows', arch: 'amd64' })).toBeNull();
    expect(preferredCliArtifact(undefined, { os: 'linux', arch: 'amd64' })).toBeNull();
    expect(preferredCliArtifact(artifacts, null)).toBeNull();
  });
});

describe('cliApiPath', () => {
  it('strips the /api/cli prefix used by the proxy', () => {
    expect(cliApiPath('/api/cli/v1/download/linux/amd64')).toBe('/v1/download/linux/amd64');
    expect(cliApiPath('/other')).toBe('/other');
  });
});

describe('linuxMacCliFilename', () => {
  it('drops the .exe suffix and defaults to horizon', () => {
    expect(linuxMacCliFilename('horizon.exe')).toBe('horizon');
    expect(linuxMacCliFilename('HORIZON.EXE')).toBe('HORIZON');
    expect(linuxMacCliFilename('horizon-linux')).toBe('horizon-linux');
    expect(linuxMacCliFilename(undefined)).toBe('horizon');
  });
});
