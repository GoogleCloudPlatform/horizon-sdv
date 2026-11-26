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
import { sanitizeMermaidSvg } from './svgSanitize';

// mermaid.render is not safe to call concurrently.
let pending: Promise<void> = Promise.resolve();

export function renderMermaidSvg(id: string, source: string, dark: boolean): Promise<string> {
  const run = pending.then(async () => {
    const { default: mermaid } = await import('mermaid');
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: dark ? 'dark' : 'default',
      fontFamily: 'Roboto, Helvetica, Arial, sans-serif',
    });
    const result = await mermaid.render(id, source);
    return sanitizeMermaidSvg(result.svg);
  });
  pending = run.then(
    () => undefined,
    () => undefined
  );
  return run;
}
