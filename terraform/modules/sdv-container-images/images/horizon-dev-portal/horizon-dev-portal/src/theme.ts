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
import { createTheme } from '@mui/material/styles';
import type { PaletteMode } from '@mui/material/styles';

/** Colours of the Documentation reader (category chip, On this page card, code and diagram blocks). */
export type DocsPalette = {
  chip: string;
  /** Chip of entries that open another application (neutral, so they stand apart from bundled guides). */
  chipExternal: string;
  contentsBorder: string;
  contentsLink: string;
  codeBackground: string;
  codeText: string;
  diagramBackground: string;
};

declare module '@mui/material/styles' {
  interface Palette {
    docs: DocsPalette;
  }
  interface PaletteOptions {
    docs?: DocsPalette;
  }
}

export const getTheme = (mode: PaletteMode) =>
  createTheme({
    palette: {
      mode,
      primary: {
        main: '#1a73e8',
        light: '#4285f4',
        dark: '#1557b0',
        contrastText: '#fff',
      },
      secondary: {
        main: '#34a853',
        light: '#81c995',
        dark: '#188038',
        contrastText: '#fff',
      },
      error: {
        main: '#ea4335',
        light: '#ee675c',
        dark: '#c5221f',
      },
      warning: {
        main: '#fbbc04',
        light: '#fdd663',
        dark: '#f29900',
      },
      info: {
        main: '#4285f4',
        light: '#669df6',
        dark: '#1967d2',
      },
      success: {
        main: '#34a853',
        light: '#81c995',
        dark: '#188038',
      },
      ...(mode === 'light'
        ? {
            background: {
              default: '#f8f9fa',
              paper: '#ffffff',
            },
            text: {
              primary: '#202124',
              secondary: '#5f6368',
            },
            docs: {
              chip: '#1a4f9c',
              chipExternal: '#5f6368',
              contentsBorder: 'rgba(0, 0, 0, 0.12)',
              contentsLink: '#1a73e8',
              codeBackground: '#0d1117',
              codeText: '#e6edf3',
              diagramBackground: '#f8f9fa',
            },
          }
        : {
            background: {
              default: '#202124',
              paper: '#292a2d',
            },
            text: {
              primary: '#e8eaed',
              secondary: '#9aa0a6',
            },
            docs: {
              chip: '#8ab4f8',
              chipExternal: '#9aa0a6',
              contentsBorder: '#3c4043',
              contentsLink: '#8ab4f8',
              codeBackground: '#0d1117',
              codeText: '#e6edf3',
              diagramBackground: '#202124',
            },
          }),
    },
    typography: {
      fontFamily: '"Roboto", "Helvetica", "Arial", sans-serif',
      button: {
        textTransform: 'none',
        fontWeight: 500,
      },
    },
    shape: {
      borderRadius: 8,
    },
    components: {
      MuiButton: {
        styleOverrides: {
          root: {
            borderRadius: 4,
            textTransform: 'none',
            fontWeight: 500,
            padding: '8px 24px',
          },
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            boxShadow: '0 1px 2px 0 rgba(60,64,67,.3), 0 1px 3px 1px rgba(60,64,67,.15)',
            borderRadius: 8,
          },
        },
      },
      MuiDrawer: {
        styleOverrides: {
          paper: {
            borderRight: '1px solid',
            borderColor: mode === 'light' ? '#e8eaed' : '#3c4043',
          },
        },
      },
    },
  });
