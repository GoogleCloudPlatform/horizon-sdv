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
import {
  Box,
  Button,
  Stack,
  Typography,
  Divider,
  CircularProgress,
  Alert,
  Paper,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  Tooltip,
  IconButton,
} from '@mui/material';
import { useCallback, useEffect, useMemo, useState } from 'react';
import DownloadIcon from '@mui/icons-material/Download';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import type { CliArtifact, CliManifest } from '../../types.ts';
import { apiCli, readCliJson } from '../../utils/api.ts';

type NavigatorUAData = {
  platform?: string;
  architecture?: string;
  getHighEntropyValues?: (hints: string[]) => Promise<{
    platform?: string;
    architecture?: string;
  }>;
};

function formatBytes(n: number): string {
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

function formatBuildDate(value: string | undefined): string {
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

function filenameFromDisposition(header: string | null, fallback: string): string {
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

async function detectPlatform(): Promise<{ os: string; arch: string }> {
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

function artifactKey(a: Pick<CliArtifact, 'os' | 'arch'>): string {
  return `${a.os}/${a.arch}`;
}

function HorizonCliTab() {
  const [manifest, setManifest] = useState<CliManifest | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [detected, setDetected] = useState<{ os: string; arch: string } | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void detectPlatform().then((p) => {
      if (!cancelled) {
        setDetected(p);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setLoadError(null);
      try {
        const resp = await apiCli('/v1/manifest');
        if (resp.status === 503) {
          let detail = 'CLI binaries are not available in this environment.';
          try {
            const body = await readCliJson<{ detail?: string }>(resp);
            if (body.detail) {
              detail = body.detail;
            }
          } catch {
            // 503 may still be plain text from the Go proxy.
          }
          throw new Error(detail);
        }
        if (!resp.ok) {
          throw new Error(`Could not load CLI catalog (${resp.status}).`);
        }
        const data = await readCliJson<CliManifest>(resp);
        if (!cancelled) {
          setManifest(data);
        }
      } catch (e) {
        if (!cancelled) {
          setLoadError(e instanceof Error ? e.message : 'Could not load CLI catalog.');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const preferred = useMemo(() => {
    if (!manifest || !detected) {
      return null;
    }
    return (
      manifest.artifacts.find((a) => a.os === detected.os && a.arch === detected.arch) ??
      manifest.artifacts.find((a) => a.os === detected.os) ??
      null
    );
  }, [manifest, detected]);

  const copySha = useCallback(async (sha: string) => {
    try {
      await navigator.clipboard.writeText(sha);
      setCopied(sha);
      window.setTimeout(() => setCopied((cur) => (cur === sha ? null : cur)), 1500);
    } catch {
      setDownloadError('Could not copy checksum.');
    }
  }, []);

  const downloadArtifact = useCallback(async (art: CliArtifact) => {
    setDownloadError(null);
    const key = artifactKey(art);
    setDownloading(key);
    try {
      const path = art.downloadPath.replace(/^\/api\/cli/, '');
      const resp = await apiCli(path);
      if (!resp.ok) {
        throw new Error(`Download failed (${resp.status}).`);
      }
      const blob = await resp.blob();
      const name = filenameFromDisposition(resp.headers.get('Content-Disposition'), art.filename);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = name;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e) {
      setDownloadError(e instanceof Error ? e.message : 'Download failed.');
    } finally {
      setDownloading(null);
    }
  }, []);

  const linuxMacFilename = (preferred?.filename ?? 'horizon').replace(/\.exe$/i, '');
  const linuxMacInstallationDescription =
    'To install the CLI system-wide and make it available from any terminal:';

  return (
    <Stack spacing={2} sx={{ mt: 4 }}>
      <Box>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Download a ready-to-run Horizon CLI binary for this cluster. You do not need a Go
          toolchain. After download, put the file on your PATH and run <code>horizon version</code>.
        </Typography>
      </Box>

      {loading && (
        <Box display="flex" justifyContent="center" py={4}>
          <CircularProgress />
        </Box>
      )}

      {loadError && (
        <Alert severity="error" role="alert">
          {loadError}
        </Alert>
      )}

      {downloadError && (
        <Alert severity="error" onClose={() => setDownloadError(null)} role="alert">
          {downloadError}
        </Alert>
      )}

      {manifest && (
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Stack spacing={2}>
            <Stack spacing={0.25} alignItems="flex-start">
              <Typography variant="subtitle1">
                <strong>Version {manifest.version}</strong>
              </Typography>
              <Divider sx={{ width: 100 }} />
              <Typography variant="caption" color="text.secondary">
                {formatBuildDate(manifest.buildDate)}
              </Typography>
            </Stack>
            {preferred && (
              <Box>
                <Button
                  variant="contained"
                  size="small"
                  startIcon={<DownloadIcon />}
                  disabled={downloading != null}
                  onClick={() => void downloadArtifact(preferred)}
                >
                  {downloading === artifactKey(preferred)
                    ? 'Downloading…'
                    : `Download for ${preferred.os} (${preferred.arch})`}
                </Button>
                <Typography
                  variant="subtitle2"
                  color="text.secondary"
                  display="block"
                  sx={{ mt: 1 }}
                >
                  Detected from this browser. Use the table below for other platforms
                </Typography>
              </Box>
            )}
            <Typography variant="subtitle1" sx={{ mt: 3 }}>
              <strong>Installation: </strong>
            </Typography>
            <Typography variant="subtitle2" color="text.secondary">
              <strong>Linux: </strong>
              {linuxMacInstallationDescription}
              <pre>
                <code>
                  chmod +x {linuxMacFilename} && sudo mv {linuxMacFilename} /usr/local/bin/
                </code>
              </pre>
            </Typography>
            <Typography variant="subtitle2" color="text.secondary">
              <strong>macOS (Apple Silicon / arm64): </strong>
              {linuxMacInstallationDescription}
              <pre>
                <code>
                  {[
                    `chmod +x ${linuxMacFilename}`,
                    `xattr -d com.apple.quarantine ${linuxMacFilename}`,
                    `sudo mv ${linuxMacFilename} /usr/local/bin/`,
                  ].join('\n')}
                </code>
              </pre>
              <strong>Note: </strong>
              The quarantine attribute is removed to avoid macOS Gatekeeper warnings for this
              unsigned binary.
            </Typography>
            <Typography variant="subtitle2" color="text.secondary">
              <strong>Windows: </strong>
              This binary is not digitally signed. Windows SmartScreen may display a warning before
              the application can be run.
            </Typography>
            <Typography variant="subtitle1">
              <strong>Verification: </strong>
            </Typography>
            <Typography variant="subtitle2" color="text.secondary">
              Run the following command to verify that the installed binary was downloaded and
              installed successfully:
            </Typography>
            <Typography
              variant="body2"
              component="div"
              sx={{ mt: 1, mb: 2, fontFamily: 'monospace' }}
            >
              <code>horizon version</code>
            </Typography>
            <Typography variant="subtitle2" sx={{ mt: 1 }} color="text.secondary">
              Expected output:
            </Typography>
            <Typography variant="body2" component="div" sx={{ mb: 3, fontFamily: 'monospace' }}>
              <strong>
                horizon {manifest.version} (built {formatBuildDate(manifest.buildDate)})
              </strong>
            </Typography>
          </Stack>
        </Paper>
      )}

      {manifest && (
        <Paper variant="outlined" sx={{ overflow: 'auto' }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>OS</TableCell>
                <TableCell>Arch</TableCell>
                <TableCell>Size</TableCell>
                <TableCell>SHA-256</TableCell>
                <TableCell align="right">Download</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {manifest.artifacts.map((art) => {
                const key = artifactKey(art);
                const isPref = preferred != null && artifactKey(preferred) === key;
                return (
                  <TableRow key={key} selected={isPref}>
                    <TableCell>{art.os}</TableCell>
                    <TableCell>{art.arch}</TableCell>
                    <TableCell>{formatBytes(art.sizeBytes)}</TableCell>
                    <TableCell>
                      <Stack direction="row" alignItems="center" spacing={0.5}>
                        <Typography
                          variant="caption"
                          component="code"
                          sx={{ wordBreak: 'break-all' }}
                        >
                          {art.sha256}
                        </Typography>
                        <Tooltip title={copied === art.sha256 ? 'Copied' : 'Copy checksum'}>
                          <IconButton
                            size="small"
                            aria-label={`Copy SHA-256 for ${art.os} ${art.arch}`}
                            onClick={() => void copySha(art.sha256)}
                          >
                            <ContentCopyIcon fontSize="inherit" />
                          </IconButton>
                        </Tooltip>
                      </Stack>
                    </TableCell>
                    <TableCell align="right">
                      <Button
                        size="small"
                        startIcon={<DownloadIcon />}
                        disabled={downloading != null}
                        onClick={() => void downloadArtifact(art)}
                      >
                        {downloading === key ? '…' : art.filename}
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Paper>
      )}
    </Stack>
  );
}

export default HorizonCliTab;
