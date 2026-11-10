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
import type { ReactElement } from 'react';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { Chip, Tooltip } from '@mui/material';
import { alpha } from '@mui/material/styles';

/** Amber role chip used on admin applications, admin-only templates, and gated controls. */
export function RoleBadge({ label }: { label: string }) {
  return (
    <Chip
      size="small"
      variant="outlined"
      icon={<LockOutlinedIcon sx={{ fontSize: 16 }} />}
      label={label}
      sx={{
        height: 24,
        fontWeight: 600,
        color: 'warning.dark',
        borderColor: 'warning.main',
        bgcolor: (theme) => alpha(theme.palette.warning.main, 0.14),
        '& .MuiChip-icon': { color: 'warning.dark', ml: 0.5 },
      }}
    />
  );
}

/** Reason tooltip. Disabled controls do not emit hover events, so the child is wrapped. */
export function HoverTooltip({
  title,
  children,
  fullWidth = false,
}: {
  title: string;
  children: ReactElement;
  fullWidth?: boolean;
}) {
  if (!title) {
    return children;
  }
  return (
    <Tooltip title={title} arrow>
      <span
        style={
          fullWidth
            ? { display: 'block', width: '100%' }
            : { display: 'inline-flex', verticalAlign: 'middle' }
        }
      >
        {children}
      </span>
    </Tooltip>
  );
}
