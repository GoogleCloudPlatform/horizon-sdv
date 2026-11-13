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
// @vitest-environment jsdom
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiCli, readCliJson } from '../../utils/api';
import { renderThemed } from '../../test/render';
import HorizonCliTab from './HorizonCliTab';

vi.mock('../../utils/api.ts', () => ({
  apiCli: vi.fn(),
  readCliJson: vi.fn(async (resp: Response) => resp.json()),
}));

vi.mock('./cliFormat.ts', async () => {
  const actual = await vi.importActual<typeof import('./cliFormat')>('./cliFormat');
  return {
    ...actual,
    detectPlatform: async () => ({ os: 'linux', arch: 'amd64' }),
  };
});

const manifest = {
  name: 'horizon',
  version: '4.3.0',
  buildDate: '2026-03-05T12:00:00.000Z',
  artifacts: [
    {
      os: 'linux',
      arch: 'amd64',
      filename: 'horizon',
      sha256: 'abc123',
      sizeBytes: 2048,
      downloadPath: '/api/cli/v1/download/linux/amd64',
    },
  ],
};

describe('HorizonCliTab', () => {
  beforeEach(() => {
    vi.mocked(apiCli).mockReset();
    vi.mocked(readCliJson).mockReset();
    vi.mocked(readCliJson).mockImplementation(async (resp: Response) => resp.json());
  });

  it('shows the catalog for the detected platform and copies a checksum', async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    vi.mocked(apiCli).mockResolvedValue(new Response(JSON.stringify(manifest), { status: 200 }));
    renderThemed(<HorizonCliTab />);

    expect(await screen.findByText('Version 4.3.0')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Download for linux (amd64)' })).toBeInTheDocument();
    expect(screen.getByText('2.0 KiB')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Copy SHA-256 for linux amd64' }));
    expect(writeText).toHaveBeenCalledWith('abc123');
  });

  it('explains when the catalog is unavailable', async () => {
    vi.mocked(apiCli).mockResolvedValue(
      new Response(JSON.stringify({ detail: 'CLI binaries are not published.' }), { status: 503 })
    );
    renderThemed(<HorizonCliTab />);
    expect(await screen.findByRole('alert')).toHaveTextContent('CLI binaries are not published.');
  });
});
