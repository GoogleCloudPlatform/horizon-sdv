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
import type { CliArtifact } from '../../types';

type NavigatorUAData = {
  platform?: string;
  architecture?: string;
  getHighEntropyValues?: (hints: string[]) => Promise<{
    platform?: string;
    architecture?: string;
  }>;
};

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) {
    return '—';
  }
  if (n < 1024) {
    return `${n} B`;
  }
  if (n < 1024 * 1024) {
    return `${(n / 1024).toFixed(1)} KiB`;
  }
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

export function formatBuildDate(value: string | undefined): string {
  if (!value) {
    return 'unknown';
  }

  const date = new Date(value);
  if (isNaN(date.getTime())) {
    return 'unknown';
  }

  return new Intl.DateTimeFormat('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(date);
}

export function filenameFromDisposition(header: string | null, fallback: string): string {
  if (!header) {
    return fallback;
  }
  const star = /filename\*=UTF-8''([^;]+)/i.exec(header);
  if (star?.[1]) {
    try {
      return decodeURIComponent(star[1]);
    } catch {
      return star[1];
    }
  }
  const quoted = /filename="([^"]+)"/i.exec(header);
  if (quoted?.[1]) {
    return quoted[1];
  }
  const plain = /filename=([^;]+)/i.exec(header);
  return plain?.[1]?.trim() || fallback;
}

export async function detectPlatform(): Promise<{ os: string; arch: string }> {
  const nav = navigator as Navigator & { userAgentData?: NavigatorUAData };
  let platform = (nav.userAgentData?.platform || navigator.platform || '').toLowerCase();
  let architecture = (nav.userAgentData?.architecture || '').toLowerCase();
  try {
    const high = await nav.userAgentData?.getHighEntropyValues?.(['platform', 'architecture']);
    if (high?.platform) {
      platform = high.platform.toLowerCase();
    }
    if (high?.architecture) {
      architecture = high.architecture.toLowerCase();
    }
  } catch {
    // UA-CH may be unavailable; fall back to userAgent.
  }
  const ua = navigator.userAgent.toLowerCase();
  let os = 'linux';
  if (platform.includes('win') || ua.includes('windows')) {
    os = 'windows';
  } else if (platform.includes('mac') || ua.includes('mac')) {
    os = 'darwin';
  }
  let arch = 'amd64';
  if (
    architecture.includes('arm') ||
    ua.includes('arm64') ||
    ua.includes('aarch64') ||
    ua.includes('apple silicon')
  ) {
    arch = 'arm64';
  }
  return { os, arch };
}

export function artifactKey(a: Pick<CliArtifact, 'os' | 'arch'>): string {
  return `${a.os}/${a.arch}`;
}

export function preferredCliArtifact<T extends { os: string; arch: string }>(
  artifacts: T[] | undefined,
  detected: { os: string; arch: string } | null
): T | null {
  if (!artifacts || !detected) {
    return null;
  }
  return (
    artifacts.find((a) => a.os === detected.os && a.arch === detected.arch) ??
    artifacts.find((a) => a.os === detected.os) ??
    null
  );
}

/** Horizon CLI download paths are served under `/api/cli`; the portal client calls that prefix itself. */
export function cliApiPath(downloadPath: string): string {
  return downloadPath.replace(/^\/api\/cli/, '');
}

export function linuxMacCliFilename(filename: string | undefined): string {
  return (filename ?? 'horizon').replace(/\.exe$/i, '');
}
