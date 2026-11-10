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
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  FormControlLabel,
  Grid,
  Stack,
  Switch,
  TextField,
  Typography,
  Chip,
} from '@mui/material';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { apiMm } from '../../utils/api';
import type { ModuleResponse, StatusResponse } from '../../types';
import { deploymentStatus } from '../../moduleStatus';
import { READY_MODULES_REFRESH_EVENT } from '../../constants';
import { usePortalAccess } from '../../rbac/AccessProvider';
import { HoverTooltip, RoleBadge } from '../../rbac/GatedControl';
import { gatedTooltip, hasGrant } from '../../rbac/portalAccess';

async function fetchModules(): Promise<ModuleResponse[]> {
  const r = await apiMm('/modules');
  if (!r.ok) {
    throw new Error(`modules: ${r.status}`);
  }
  return r.json() as Promise<ModuleResponse[]>;
}

async function fetchStatus(idOrName: string): Promise<StatusResponse> {
  const r = await apiMm(`/modules/${encodeURIComponent(idOrName)}/status`);
  if (!r.ok) {
    throw new Error(`status: ${r.status}`);
  }
  return r.json() as Promise<StatusResponse>;
}

type RefreshResult =
  { ok: true; list: ModuleResponse[]; allReady: boolean } | { ok: false; allReady: boolean };

export function ModulesTab() {
  const access = usePortalAccess();
  const canModules = hasGrant(access, 'platform.modules');
  const moduleTip = !access
    ? 'Checking your access…'
    : canModules
      ? ''
      : gatedTooltip(access, 'platform.modules');
  const [mods, setMods] = useState<ModuleResponse[]>([]);
  const [statuses, setStatuses] = useState<Record<string, StatusResponse>>({});
  const [refDraft, setRefDraft] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  /** Name of the module whose pin editor is open, or null when all editors are closed. */
  const [pinEditor, setPinEditor] = useState<string | null>(null);
  /** Ref copy of `busy` for the polling loop and visibility refresh. */
  const busyRef = useRef<string | null>(null);
  /** When true, background poll skips `refresh()` during long enable (reduces READY→INSTALLATION flicker). Disable/apply-ref keep polling so READY→UNINSTALL updates while DELETE runs. */
  const suppressBackgroundPollRef = useRef(false);
  busyRef.current = busy;

  const cancelPinEditor = (m: ModuleResponse) => {
    setRefDraft((prev) => ({ ...prev, [m.name]: m.pinned ? (m.targetRevision ?? '').trim() : '' }));
    setPinEditor(null);
  };

  const refresh = useCallback(async (): Promise<RefreshResult> => {
    try {
      const list = await fetchModules();
      setRefDraft((prev) => {
        const next = { ...prev };
        for (const m of list) {
          if (next[m.name] === undefined) {
            // Only pre-fill the field for pinned modules (so Apply ref shows the current pin).
            // Following/disabled modules get an empty string so enable sends no body and follows
            // the platform branch rather than accidentally pinning it.
            next[m.name] = m.pinned ? (m.targetRevision ?? '').trim() : '';
          }
        }
        return next;
      });
      const st: Record<string, StatusResponse> = {};
      await Promise.all(
        list.map(async (m) => {
          try {
            st[m.name] = await fetchStatus(m.name);
          } catch {
            st[m.name] = {};
          }
        })
      );
      // Disable can commit on the server between GET /modules and GET /modules/{name}/status; the list would
      // still show enabled=true while Argo reports OutOfSync+Healthy for the pruning parent, which wrongly maps
      // to UPDATE IN PROGRESS. Re-fetch the module list so enabled matches the latest ModuleManagerState.
      const listFinal = await fetchModules();
      setMods(listFinal);
      setStatuses(st);
      setError(null);
      const allReady = listFinal.every((m) => {
        const label = deploymentStatus(m.enabled, st[m.name]);
        if (!m.enabled) {
          return label === 'NOT INSTALLED';
        }
        return label === 'READY';
      });
      return { ok: true, list: listFinal, allReady };
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Load failed');
      return { ok: false, allReady: true };
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    let timeoutId = 0;

    const schedule = (delay: number) => {
      timeoutId = window.setTimeout(async () => {
        if (cancelled) {
          return;
        }
        if (busyRef.current && suppressBackgroundPollRef.current) {
          schedule(3000);
          return;
        }
        const res = await refresh();
        if (cancelled) {
          return;
        }
        const allReady = res.ok ? res.allReady : false;
        // Steady state still polls so out-of-band disables (Argo/kubectl) update without a full reload.
        schedule(allReady ? 5000 : 3000);
      }, delay);
    };

    schedule(0);
    return () => {
      cancelled = true;
      window.clearTimeout(timeoutId);
    };
  }, [refresh]);

  useEffect(() => {
    const onVis = () => {
      if (
        document.visibilityState !== 'visible' ||
        (busyRef.current && suppressBackgroundPollRef.current)
      ) {
        return;
      }
      void refresh();
    };
    document.addEventListener('visibilitychange', onVis);
    return () => document.removeEventListener('visibilitychange', onVis);
  }, [refresh]);

  const resetToPlatformBranch = async (m: ModuleResponse) => {
    if (!hasGrant(access, 'platform.modules')) {
      return;
    }
    setBusy(m.name);
    suppressBackgroundPollRef.current = false;
    setError(null);
    try {
      const r = await apiMm(`/modules/${encodeURIComponent(m.name)}/target-revision`, {
        method: 'DELETE',
      });
      if (!r.ok) {
        const t = await r.text();
        throw new Error(t || `reset ref ${r.status}`);
      }
      // Clear the draft so the field reverts to the platform-branch placeholder, not the stale pin.
      setRefDraft((prev) => ({ ...prev, [m.name]: '' }));
      setPinEditor(null);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Request failed');
    } finally {
      setBusy(null);
      suppressBackgroundPollRef.current = false;
      window.dispatchEvent(new CustomEvent(READY_MODULES_REFRESH_EVENT));
    }
  };

  const applyRef = async (m: ModuleResponse) => {
    if (!hasGrant(access, 'platform.modules')) {
      return;
    }
    const ref = (refDraft[m.name] ?? '').trim();
    if (!ref) {
      setError('Git ref cannot be empty');
      return;
    }
    setBusy(m.name);
    suppressBackgroundPollRef.current = false;
    setError(null);
    try {
      const r = await apiMm(`/modules/${encodeURIComponent(m.name)}/target-revision`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ targetRevision: ref }),
      });
      if (!r.ok) {
        const t = await r.text();
        throw new Error(t || `set ref ${r.status}`);
      }
      setPinEditor(null);
      await refresh();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Request failed');
    } finally {
      setBusy(null);
      suppressBackgroundPollRef.current = false;
      window.dispatchEvent(new CustomEvent(READY_MODULES_REFRESH_EVENT));
    }
  };

  /** Module Manager can return 200 on enable/disable before the next GET /modules observes the new flag (informer/cache). */
  const refreshUntilModuleEnabledMatches = async (moduleName: string, wantEnabled: boolean) => {
    for (let i = 0; i < 15; i++) {
      const res = await refresh();
      if (!res.ok) {
        return;
      }
      const row = res.list.find((x) => x.name === moduleName);
      if (row && row.enabled === wantEnabled) {
        return;
      }
      await new Promise((r) => setTimeout(r, 400));
    }
  };

  const toggle = async (m: ModuleResponse, enable: boolean) => {
    if (!hasGrant(access, 'platform.modules')) {
      return;
    }
    setBusy(m.name);
    suppressBackgroundPollRef.current = enable;
    setError(null);
    try {
      if (enable) {
        const refTrim = (refDraft[m.name] ?? '').trim();
        const init: RequestInit = {
          method: 'POST',
          headers: refTrim ? { 'Content-Type': 'application/json' } : undefined,
          body: refTrim ? JSON.stringify({ targetRevision: refTrim }) : undefined,
        };
        const r = await apiMm(`/modules/${encodeURIComponent(m.name)}/enable`, init);
        if (!r.ok) {
          const t = await r.text();
          throw new Error(t || `enable ${r.status}`);
        }
        setPinEditor(null);
        await refreshUntilModuleEnabledMatches(m.name, true);
      } else {
        const r = await apiMm(`/modules/${encodeURIComponent(m.name)}/disable`, {
          method: 'DELETE',
        });
        if (r.status === 409) {
          const j = (await r.json()) as { hardDependents?: string[] };
          throw new Error(
            `Cannot disable: required by: ${(j.hardDependents ?? []).join(', ') || 'other modules'}`
          );
        }
        if (!r.ok) {
          const t = await r.text();
          throw new Error(t || `disable ${r.status}`);
        }
        // Clear the draft so a subsequent re-enable follows the platform branch by default.
        setRefDraft((prev) => ({ ...prev, [m.name]: '' }));
        setPinEditor(null);
        await refreshUntilModuleEnabledMatches(m.name, false);
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Request failed');
    } finally {
      setBusy(null);
      suppressBackgroundPollRef.current = false;
      window.dispatchEvent(new CustomEvent(READY_MODULES_REFRESH_EVENT));
    }
  };

  if (loading && mods.length === 0) {
    return (
      <Box display="flex" justifyContent="center" p={4}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Stack spacing={1.25} sx={{ mb: 2 }}>
        <Typography variant="body2" color="text.secondary" component="p" sx={{ m: 0 }}>
          Enable or disable modules. Hard dependencies are enabled automatically. Enabling follows
          the <strong>platform branch</strong> (the cluster default from Terraform) by default. Use{' '}
          <strong>Pin to ref</strong> on an enabled module, or{' '}
          <strong>Pin ref before install</strong> on a disabled one, to lock it to a specific
          branch, tag, or commit. Use <strong>Reset to platform branch</strong> on a pinned module
          to follow the platform again. Dependent modules are listed on each card.
        </Typography>
        <Typography variant="body2" color="text.secondary" component="p" sx={{ m: 0 }}>
          Status chips reflect Module Manager and Argo CD (install, update, uninstall, or ready).
          First-time installs and uninstalls can take several minutes; disable may wait while
          related cloud resources finish tearing down.
        </Typography>
        <Typography variant="body2" color="text.secondary" component="p" sx={{ m: 0 }}>
          When no toggle or apply is running, this page refreshes status on its own every few
          seconds. Bringing this tab back to the foreground also triggers a refresh.
        </Typography>
      </Stack>
      <Alert severity="warning" sx={{ mb: 2 }}>
        <Stack spacing={1}>
          <Typography variant="body2" component="div">
            <strong>Do not reload or close this tab</strong> while enable, disable, pin / apply ref,
            or &quot;Reset to platform branch&quot; is running.
          </Typography>
          <Typography variant="body2" component="div">
            A full page refresh usually <strong>cancels the in-flight browser request</strong>. The
            server may still apply or partially apply the change, so the UI can disagree with the
            cluster until you load fresh data <em>after</em> the operation completes.
          </Typography>
          <Typography variant="body2" component="div">
            While this page is idle, status updates every few seconds automatically. Only
            hard-refresh if you believe the view is stale and no command is in progress.
          </Typography>
        </Stack>
      </Alert>
      {access && !canModules && (
        <Alert severity="info" icon={<LockOutlinedIcon />} sx={{ mb: 2 }}>
          <strong>View only.</strong> You are signed in as {access.roleLabel}. Enabling, disabling,
          pinning and resetting modules needs Admin, because these change shared cloud resources for
          everyone.
        </Alert>
      )}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      <Grid container spacing={2}>
        {mods.map((m) => {
          const st = statuses[m.name];
          const label = deploymentStatus(m.enabled, st);
          const moduleUiLocked =
            busy === m.name ||
            label === 'UNINSTALL IN PROGRESS' ||
            label === 'INSTALLATION IN PROGRESS' ||
            label === 'UPDATE IN PROGRESS';
          const controlsLocked = moduleUiLocked || !canModules;
          const color =
            label === 'READY'
              ? 'success'
              : label === 'NOT INSTALLED' || label === 'DEPLOYMENT PENDING'
                ? 'default'
                : label === 'UPDATE IN PROGRESS'
                  ? 'info'
                  : 'warning';
          return (
            <Grid size={{ xs: 12, sm: 6, md: 4 }} key={m.id || m.name}>
              <Card variant="outlined">
                <CardContent>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      justifyContent: 'space-between',
                      gap: 1,
                    }}
                  >
                    <Typography variant="h6">{m.name}</Typography>
                    {access && !canModules ? <RoleBadge label="Admin" /> : null}
                  </Box>
                  <Box
                    sx={{ my: 1, display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}
                  >
                    <Chip size="small" label={label} color={color} />
                  </Box>
                  {label === 'DEPLOYMENT PENDING' ? (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ mb: 0.5 }}
                    >
                      Enabled in Module Manager, but no matching Argo CD Application was found
                      (reconcile lag, manual delete, or different cluster/namespace than
                      module-manager). Disable if this is unintended.
                    </Typography>
                  ) : null}
                  {m.enabled ? (
                    <Box
                      sx={{
                        mb: 0.5,
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        flexWrap: 'wrap',
                      }}
                    >
                      <Chip
                        size="small"
                        variant="outlined"
                        label={m.pinned ? 'Pinned' : 'Following platform'}
                        color={m.pinned ? 'warning' : 'info'}
                      />
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        component="span"
                        title={(m.pinned ? m.targetRevision : m.clusterTargetRevision) || ''}
                        sx={{
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          maxWidth: '100%',
                        }}
                      >
                        {(m.pinned ? m.targetRevision : m.clusterTargetRevision) || '\u2014'}
                      </Typography>
                    </Box>
                  ) : null}
                  {pinEditor === m.name ? (
                    <Box sx={{ mt: 1 }}>
                      <TextField
                        size="small"
                        fullWidth
                        label="Custom Git ref"
                        placeholder={m.clusterTargetRevision || 'branch / tag / SHA'}
                        value={refDraft[m.name] ?? ''}
                        disabled={controlsLocked}
                        onChange={(e) =>
                          setRefDraft((prev) => ({ ...prev, [m.name]: e.target.value }))
                        }
                        helperText={
                          m.enabled
                            ? m.pinned
                              ? 'Edit and Apply to change the pin, or Reset to follow the platform branch.'
                              : 'Enter a branch, tag, or commit to pin. Cancel to keep following the platform branch.'
                            : 'Optional. Pins this module to this ref on install. Leave empty to follow the platform branch.'
                        }
                      />
                      <Box sx={{ mt: 1, display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                        {m.enabled ? (
                          <HoverTooltip title={moduleTip}>
                            <Button
                              size="small"
                              variant="contained"
                              disabled={controlsLocked}
                              startIcon={!canModules ? <LockOutlinedIcon /> : undefined}
                              onClick={() => void applyRef(m)}
                            >
                              Apply
                            </Button>
                          </HoverTooltip>
                        ) : null}
                        <Button
                          size="small"
                          variant="outlined"
                          disabled={moduleUiLocked}
                          onClick={() => cancelPinEditor(m)}
                        >
                          Cancel
                        </Button>
                      </Box>
                    </Box>
                  ) : (
                    <Box sx={{ mt: 1, display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                      {m.enabled ? (
                        <>
                          <HoverTooltip title={moduleTip}>
                            <Button
                              size="small"
                              variant="outlined"
                              disabled={controlsLocked}
                              startIcon={!canModules ? <LockOutlinedIcon /> : undefined}
                              onClick={() => setPinEditor(m.name)}
                            >
                              {m.pinned ? 'Edit ref' : 'Pin to ref'}
                            </Button>
                          </HoverTooltip>
                          {m.pinned ? (
                            <HoverTooltip title={moduleTip}>
                              <Button
                                size="small"
                                variant="outlined"
                                color="secondary"
                                disabled={controlsLocked}
                                startIcon={!canModules ? <LockOutlinedIcon /> : undefined}
                                onClick={() => void resetToPlatformBranch(m)}
                              >
                                Reset to platform branch
                              </Button>
                            </HoverTooltip>
                          ) : null}
                        </>
                      ) : (
                        <HoverTooltip title={moduleTip}>
                          <Button
                            size="small"
                            variant="text"
                            disabled={controlsLocked}
                            startIcon={!canModules ? <LockOutlinedIcon /> : undefined}
                            onClick={() => setPinEditor(m.name)}
                            sx={{ px: 0, minWidth: 0 }}
                          >
                            Pin ref before install
                          </Button>
                        </HoverTooltip>
                      )}
                    </Box>
                  )}
                  <HoverTooltip title={moduleTip}>
                    <FormControlLabel
                      sx={{ mt: 1, display: 'inline-flex' }}
                      control={
                        <Switch
                          checked={m.enabled}
                          disabled={controlsLocked}
                          onChange={(_, v) => void toggle(m, v)}
                        />
                      }
                      label={
                        <Box
                          component="span"
                          sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5 }}
                        >
                          {!canModules ? <LockOutlinedIcon sx={{ fontSize: 16 }} /> : null}
                          {m.enabled ? 'Enabled' : 'Disabled'}
                        </Box>
                      }
                    />
                  </HoverTooltip>
                  {m.hardDependencies?.length || m.softDependencies?.length ? (
                    <Typography
                      variant="caption"
                      display="block"
                      color="text.secondary"
                      sx={{ mt: 1 }}
                    >
                      {m.hardDependencies?.length ? (
                        <>Hard deps: {m.hardDependencies.join(', ')}. </>
                      ) : null}
                      {m.softDependencies?.length ? (
                        <>Soft deps: {m.softDependencies.join(', ')}</>
                      ) : null}
                    </Typography>
                  ) : null}
                  {m.enabled && (m.hardDependents?.length || m.softDependents?.length) ? (
                    <Typography variant="caption" display="block" color="text.secondary">
                      {m.hardDependents?.length ? (
                        <>Hard dependents: {m.hardDependents.join(', ')}. </>
                      ) : null}
                      {m.softDependents?.length ? (
                        <>Soft dependents: {m.softDependents.join(', ')}</>
                      ) : null}
                    </Typography>
                  ) : null}
                </CardContent>
              </Card>
            </Grid>
          );
        })}
      </Grid>
    </Box>
  );
}
