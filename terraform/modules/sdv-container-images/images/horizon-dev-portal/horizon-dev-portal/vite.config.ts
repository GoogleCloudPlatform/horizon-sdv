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
import fs from 'node:fs';
import type { IncomingMessage, ServerResponse } from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';

type CliArtifact = {
  os: string;
  arch: string;
  filename: string;
  sha256: string;
  sizeBytes: number;
  downloadPath: string;
};

type CliManifest = {
  name: string;
  version: string;
  buildDate: string;
  artifacts: CliArtifact[];
};

function cliDistDir(): string {
  const fromEnv = process.env.CLI_DIST_DIR?.trim();
  if (fromEnv) {
    return fromEnv;
  }
  return path.join(os.tmpdir(), 'horizon-cli-dist');
}

function sendJson(res: ServerResponse, status: number, body: unknown): void {
  res.statusCode = status;
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  res.setHeader('Cache-Control', 'private, no-store');
  res.end(JSON.stringify(body));
}

function readManifest(distDir: string): CliManifest | null {
  try {
    const raw = fs.readFileSync(path.join(distDir, 'manifest.json'), 'utf8');
    const m = JSON.parse(raw) as CliManifest;
    if (!m?.name || !Array.isArray(m.artifacts) || m.artifacts.length === 0) {
      return null;
    }
    return m;
  } catch {
    return null;
  }
}

/**
 * Serve GET /api/cli/* from CLI_DIST_DIR during `npm run dev`.
 * Without this, a down/missing Go proxy lets Vite's SPA fallback return index.html
 * (HTTP 200), and the Tools page fails with "Unexpected token '<'".
 */
function horizonCliDevApi(): Plugin {
  return {
    name: 'horizon-cli-dev-api',
    configureServer(server) {
      const distDir = cliDistDir();
      server.config.logger.info(`[horizon-cli] dev API from ${distDir}`);
      server.middlewares.use((req: IncomingMessage, res: ServerResponse, next: () => void) => {
        if (req.method !== 'GET' && req.method !== 'HEAD') {
          next();
          return;
        }
        const pathOnly = (req.url ?? '').split('?')[0];
        const apiIdx = pathOnly.indexOf('/api/cli/');
        if (apiIdx < 0) {
          next();
          return;
        }
        const apiPath = pathOnly.slice(apiIdx);
        const manifest = readManifest(distDir);
        if (!manifest) {
          sendJson(res, 503, {
            error: 'cli binaries unavailable',
            detail: `No manifest.json under ${distDir}. From tools/horizon run: go run mkdist.go "${distDir}" <horizon_version>, e.g.: go run mkdist.go "${distDir}" 4.3.0`,
          });
          return;
        }
        if (apiPath === '/api/cli/v1/manifest') {
          sendJson(res, 200, manifest);
          return;
        }
        const dl = /^\/api\/cli\/v1\/download\/([a-z0-9]+)\/([a-z0-9]+)$/.exec(apiPath);
        if (!dl) {
          sendJson(res, 404, { error: 'not found' });
          return;
        }
        const art = manifest.artifacts.find((a) => a.os === dl[1] && a.arch === dl[2]);
        if (!art || path.basename(art.filename) !== art.filename) {
          sendJson(res, 404, { error: 'not found' });
          return;
        }
        const filePath = path.join(distDir, art.os, art.arch, art.filename);
        if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
          sendJson(res, 404, { error: 'not found' });
          return;
        }
        res.statusCode = 200;
        res.setHeader('Content-Type', 'application/octet-stream');
        res.setHeader('Content-Disposition', `attachment; filename="${art.filename}"`);
        res.setHeader('X-Checksum-SHA256', art.sha256);
        res.setHeader('Cache-Control', 'private, no-store');
        if (req.method === 'HEAD') {
          res.end();
          return;
        }
        fs.createReadStream(filePath)
          .on('error', () => {
            if (!res.headersSent) {
              sendJson(res, 500, { error: 'read failed' });
            } else {
              res.destroy();
            }
          })
          .pipe(res);
      });
    },
  };
}

/** Relative base so the same build can be mounted at any HTTP path (set at runtime via config.js `publicPath`). */
export default defineConfig({
  base: './',
  server: {
    proxy: {
      '/auth': {
        target: process.env.VITE_KEYCLOAK_ORIGIN || 'http://localhost:32080',
        changeOrigin: true,
      },
      '/api': {
        target: process.env.VITE_DEV_PROXY_TARGET || 'http://127.0.0.1:7090',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('error', (err, _req, res) => {
            const sock = res as ServerResponse;
            if (sock && !sock.headersSent && typeof sock.writeHead === 'function') {
              sock.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
              sock.end(
                JSON.stringify({
                  error: 'dev proxy target unreachable',
                  detail: err.message,
                })
              );
            }
          });
        },
      },
    },
  },
  plugins: [horizonCliDevApi(), react()],
});
