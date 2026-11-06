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
import { useState } from 'react';
import type { ReactNode } from 'react';
import ArrowDropDownIcon from '@mui/icons-material/ArrowDropDown';
import CheckIcon from '@mui/icons-material/Check';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import PersonOutlineIcon from '@mui/icons-material/PersonOutline';
import { Box, Button, Chip, Popover, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { usePortalAccess } from './AccessProvider';
import type { PortalRole } from './portalAccess';

/** Header role chip. Opens the permission matrix for the signed-in Keycloak role. */
export function YourAccessButton() {
  const access = usePortalAccess();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  if (!access) {
    return null;
  }
  const open = Boolean(anchor);
  return (
    <>
      <Button
        size="small"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={`Your role is ${access.roleLabel}. Open your access`}
        onClick={(event) => setAnchor(event.currentTarget)}
        startIcon={<PersonOutlineIcon />}
        endIcon={<ArrowDropDownIcon />}
        sx={{
          mr: { xs: 0.5, sm: 1.5 },
          color: 'primary.main',
          bgcolor: 'common.white',
          borderRadius: 999,
          textTransform: 'none',
          fontWeight: 600,
          px: { xs: 1, sm: 1.25 },
          flexShrink: 0,
          minWidth: 0,
          '& .MuiButton-startIcon': { display: { xs: 'none', sm: 'inline-flex' } },
          '&:hover': { bgcolor: 'grey.100' },
        }}
      >
        {access.roleLabel}
      </Button>
      <Popover
        open={open}
        anchorEl={anchor}
        onClose={() => setAnchor(null)}
        marginThreshold={8}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{
          paper: {
            sx: {
              mt: 1,
              width: { xs: 'calc(100vw - 16px)', sm: 460 },
              maxWidth: 'calc(100vw - 16px)',
              maxHeight: 'calc(100dvh - 24px)',
              overflow: 'auto',
              borderRadius: 2,
            },
          },
        }}
      >
        <YourAccessPanel />
      </Popover>
    </>
  );
}

function YourAccessPanel() {
  const access = usePortalAccess();
  if (!access) {
    return null;
  }
  const signedIn = access.email || access.subject;
  return (
    <Box sx={{ p: { xs: 1.5, sm: 2.25 } }}>
      <Typography variant="h6" sx={{ fontWeight: 700, mb: 1 }}>
        Your access
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1.25 }}>
        Signed in as {signedIn}
      </Typography>
      <Box sx={{ display: 'flex', gap: 0.75, flexWrap: 'wrap', mb: 1.25 }}>
        {access.roles.map((role) => {
          const current = role.id === access.role;
          return (
            <Chip
              key={role.id}
              size="small"
              label={current ? `${role.label} (you)` : role.label}
              color={current ? 'primary' : 'default'}
              variant={current ? 'filled' : 'outlined'}
            />
          );
        })}
      </Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
        {access.summary}
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: `minmax(0, 1fr) repeat(${Math.max(access.roles.length, 1)}, minmax(44px, 64px))`,
          alignItems: 'center',
          columnGap: 0.5,
          rowGap: 0.75,
        }}
      >
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{
            fontWeight: 600,
            position: 'sticky',
            left: 0,
            zIndex: 1,
            bgcolor: 'background.paper',
          }}
        >
          Function
        </Typography>
        {access.roles.map((role) => (
          <AccessCell key={role.id} current={role.id === access.role} header>
            <Typography
              variant="caption"
              sx={{
                fontWeight: 600,
                fontSize: '0.68rem',
                lineHeight: 1.1,
                textAlign: 'center',
                px: 0.25,
              }}
            >
              {role.label}
            </Typography>
          </AccessCell>
        ))}
        {access.sections.map((section) => (
          <SectionRows
            key={section.id}
            section={section}
            current={access.role}
            roles={access.roles.map((role) => role.id)}
          />
        ))}
      </Box>
      <Box
        sx={{
          mt: 1.75,
          px: 1.25,
          py: 1,
          borderRadius: 1,
          bgcolor: (theme) => alpha(theme.palette.text.primary, 0.04),
        }}
      >
        <Typography variant="caption" color="text.secondary">
          {access.moreAccess}
        </Typography>
      </Box>
    </Box>
  );
}

function SectionRows({
  section,
  current,
  roles,
}: {
  section: {
    title: string;
    functions: { id: string; label: string; grants: Record<PortalRole, boolean> }[];
  };
  current: PortalRole;
  roles: PortalRole[];
}) {
  return (
    <>
      <Typography
        variant="caption"
        color="primary"
        sx={{
          gridColumn: '1 / -1',
          fontWeight: 700,
          letterSpacing: 0.4,
          mt: 1,
          overflowWrap: 'anywhere',
        }}
      >
        {section.title}
      </Typography>
      {section.functions.map((fn) => (
        <FunctionRow
          key={fn.id}
          label={fn.label}
          grants={fn.grants}
          current={current}
          roles={roles}
        />
      ))}
    </>
  );
}

function FunctionRow({
  label,
  grants,
  current,
  roles,
}: {
  label: string;
  grants: Record<PortalRole, boolean>;
  current: PortalRole;
  roles: PortalRole[];
}) {
  return (
    <>
      <Typography
        variant="body2"
        sx={{
          pr: 1,
          py: 0.25,
          position: 'sticky',
          left: 0,
          zIndex: 1,
          bgcolor: 'background.paper',
          overflowWrap: 'anywhere',
        }}
      >
        {label}
      </Typography>
      {roles.map((role) => (
        <AccessCell key={role} current={role === current}>
          {grants[role] ? (
            <CheckIcon sx={{ fontSize: 18, color: 'success.main' }} aria-label="allowed" />
          ) : (
            <LockOutlinedIcon sx={{ fontSize: 16, color: 'text.disabled' }} aria-label="denied" />
          )}
        </AccessCell>
      ))}
    </>
  );
}

function AccessCell({
  current,
  header = false,
  children,
}: {
  current: boolean;
  header?: boolean;
  children: ReactNode;
}) {
  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: header ? 28 : 24,
        borderRadius: 0.75,
        bgcolor: (theme) => (current ? alpha(theme.palette.primary.main, 0.1) : 'transparent'),
      }}
    >
      {children}
    </Box>
  );
}
