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

// Dark palette that mirrors Mermaid's default light theme (lavender nodes, yellow notes and
// subgraphs), so a diagram reads the same in both modes. Mermaid's built-in "dark" theme makes
// nodes near-black on the near-black page, with thin grey lines.
const DARK_THEME_VARIABLES = {
  darkMode: true,
  background: '#202124',
  textColor: '#e8eaed',
  lineColor: '#c4c7cc',
  // Flowchart nodes, subgraphs and edge labels
  primaryColor: '#2f3558',
  primaryBorderColor: '#9aa0ff',
  primaryTextColor: '#e8eaed',
  mainBkg: '#2f3558',
  nodeBorder: '#9aa0ff',
  clusterBkg: '#37361c',
  clusterBorder: '#b8b352',
  titleColor: '#e8eaed',
  edgeLabelBackground: '#2b2d31',
  // Sequence diagrams
  actorBkg: '#2f3558',
  actorBorder: '#9aa0ff',
  actorTextColor: '#e8eaed',
  actorLineColor: '#7c82b8',
  signalColor: '#c4c7cc',
  signalTextColor: '#e8eaed',
  labelBoxBkgColor: '#2f3558',
  labelBoxBorderColor: '#9aa0ff',
  labelTextColor: '#e8eaed',
  loopTextColor: '#e8eaed',
  noteBkgColor: '#4a4520',
  noteBorderColor: '#b8b352',
  noteTextColor: '#f5f1c8',
  activationBkgColor: '#3d4370',
  activationBorderColor: '#9aa0ff',
  sequenceNumberColor: '#202124',
};

// mermaid.render is not safe to call concurrently.
let pending: Promise<void> = Promise.resolve();

export function renderMermaidSvg(id: string, source: string, dark: boolean): Promise<string> {
  const run = pending.then(async () => {
    const { default: mermaid } = await import('mermaid');
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: dark ? 'base' : 'default',
      themeVariables: dark ? DARK_THEME_VARIABLES : {},
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
