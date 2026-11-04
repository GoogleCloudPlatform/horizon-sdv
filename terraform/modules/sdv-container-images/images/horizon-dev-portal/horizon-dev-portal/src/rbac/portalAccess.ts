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
/**
 * Contract for GET /v1/portal/access (portal path /api/horizon/v1/portal/access).
 * Horizon API will serve this in the testing phase. Until then the portal proxy
 * (and the Vite dev server) return the same payload, built from the caller's
 * Keycloak groups. The UI renders this payload; it does not hard-code the matrix.
 */

export type PortalRole = 'viewer' | 'developer' | 'admin';

export interface PortalIdentity {
  subject: string;
  email: string;
  groups: string[];
}

export interface PortalFunction {
  id: string;
  label: string;
  grants: Record<PortalRole, boolean>;
  /** Action reason. The UI adds the role prefix and the "ask an administrator" sentence. */
  denial?: string;
  /** Full tooltip used when the signed-in role is not granted. Skips the generic wrapper. */
  tooltip?: string;
  requiredRole?: PortalRole;
}

export interface PortalSection {
  id: string;
  title: string;
  functions: PortalFunction[];
}

export interface AdminOnlyTemplate {
  module: string;
  pattern: string;
  label: string;
  /** Full tooltip, including the "Admin only." prefix. */
  denial: string;
}

export interface PortalApplication {
  id: string;
  audience: 'developer' | 'admin';
}

export interface PortalAccess {
  mock?: boolean;
  subject: string;
  email: string;
  role: PortalRole;
  roleLabel: string;
  keycloakGroup: string;
  summary: string;
  moreAccess: string;
  roles: { id: PortalRole; label: string }[];
  sections: PortalSection[];
  applications: PortalApplication[];
  adminOnlyTemplates: AdminOnlyTemplate[];
}

export const PORTAL_ROLES: { id: PortalRole; label: string }[] = [
  { id: 'viewer', label: 'Viewer' },
  { id: 'developer', label: 'Developer' },
  { id: 'admin', label: 'Admin' },
];

export const ROLE_LABEL: Record<PortalRole, string> = {
  viewer: 'Viewer',
  developer: 'Developer',
  admin: 'Admin',
};

const ROLE_RANK: Record<PortalRole, number> = { viewer: 1, developer: 2, admin: 3 };

const ROLE_BY_GROUP: Record<string, PortalRole> = {
  administrators: 'admin',
  admins: 'admin',
  admin: 'admin',
  'horizon-administrators': 'admin',
  developers: 'developer',
  developer: 'developer',
  'horizon-developers': 'developer',
  viewers: 'viewer',
  viewer: 'viewer',
  'horizon-viewers': 'viewer',
};

const CANONICAL_GROUP: Record<PortalRole, string> = {
  admin: 'administrators',
  developer: 'developers',
  viewer: 'viewers',
};

const MORE_ACCESS =
  'Need more access? Ask a Horizon administrator to add you to the developers or administrators group.';

const ADMIN_ONLY_TEMPLATES: AdminOnlyTemplate[] = [
  {
    module: 'workloads-android',
    pattern: 'cf-instance-template-*-delete',
    label: 'cf-instance-template-*-delete',
    denial:
      'Admin only. Deletes the shared Cuttlefish instance template for everyone. You can open the form and read it, but Submit is disabled for Developers.',
  },
  {
    module: 'remotive-topology',
    pattern: 'remotiv-instance-template-delete',
    label: 'remotiv-instance-template-delete',
    denial:
      'Admin only. Deletes the shared Remotive instance template for everyone. You can open the form and read it, but Submit is disabled for Developers.',
  },
  {
    module: 'remotive-topology',
    pattern: 'remotiv-builder-image',
    label: 'remotiv-builder-image',
    denial:
      'Admin only. Publishes the shared Remotive builder image for everyone. You can open the form and read it, but Submit is disabled for Developers.',
  },
];

const APPLICATIONS: PortalApplication[] = [
  { id: 'gerrit', audience: 'developer' },
  { id: 'jenkins', audience: 'developer' },
  { id: 'mtk-connect', audience: 'developer' },
  { id: 'mcp-gateway', audience: 'developer' },
  { id: 'argo-workflows', audience: 'developer' },
  { id: 'keycloak', audience: 'admin' },
  { id: 'argo-cd', audience: 'admin' },
  { id: 'headlamp', audience: 'admin' },
  { id: 'grafana', audience: 'admin' },
];

function normalizeGroup(raw: string): string {
  let name = raw.trim().toLowerCase();
  if (name.startsWith('/')) {
    name = name.slice(1);
  }
  const slash = name.lastIndexOf('/');
  if (slash >= 0) {
    name = name.slice(slash + 1);
  }
  return name;
}

function asRole(value: string | null | undefined): PortalRole | null {
  const name = (value ?? '').trim().toLowerCase();
  if (name === 'viewer' || name === 'developer' || name === 'admin') {
    return name;
  }
  return null;
}

export function resolveRole(
  groups: string[],
  roleOverride?: string | null
): { role: PortalRole; keycloakGroup: string; defaulted: boolean } {
  const override = asRole(roleOverride);
  if (override) {
    return { role: override, keycloakGroup: CANONICAL_GROUP[override], defaulted: false };
  }
  let best: PortalRole | null = null;
  for (const group of groups) {
    const role = ROLE_BY_GROUP[normalizeGroup(group)];
    if (!role) {
      continue;
    }
    if (!best || ROLE_RANK[role] > ROLE_RANK[best]) {
      best = role;
    }
  }
  if (!best) {
    return { role: 'developer', keycloakGroup: 'developers', defaulted: true };
  }
  return { role: best, keycloakGroup: CANONICAL_GROUP[best], defaulted: false };
}

function collectStrings(value: unknown, into: string[]): void {
  if (typeof value === 'string') {
    if (value.trim()) {
      into.push(value);
    }
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      collectStrings(item, into);
    }
  }
}

function claimString(claims: Record<string, unknown>, key: string): string {
  const value = claims[key];
  return typeof value === 'string' ? value.trim() : '';
}

export function identityFromClaims(claims: Record<string, unknown>): PortalIdentity {
  const email = claimString(claims, 'email');
  const subject =
    claimString(claims, 'preferred_username') ||
    claimString(claims, 'name') ||
    claimString(claims, 'sub') ||
    email;
  const groups: string[] = [];
  collectStrings(claims.groups, groups);
  const realm = claims.realm_access;
  if (realm && typeof realm === 'object') {
    collectStrings((realm as { roles?: unknown }).roles, groups);
  }
  return { subject, email: email || subject, groups };
}

export function identityFromToken(token: string | undefined): PortalIdentity {
  if (!token) {
    return { subject: '', email: '', groups: [] };
  }
  try {
    const part = token.split('.')[1];
    if (!part) {
      return { subject: '', email: '', groups: [] };
    }
    let b64 = part.replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4) {
      b64 += '=';
    }
    const json = JSON.parse(atob(b64)) as Record<string, unknown>;
    return identityFromClaims(json);
  } catch {
    return { subject: '', email: '', groups: [] };
  }
}

function templateSections(rules: AdminOnlyTemplate[]): PortalSection[] {
  const order: string[] = [];
  const byModule = new Map<string, AdminOnlyTemplate[]>();
  for (const rule of rules) {
    const list = byModule.get(rule.module);
    if (list) {
      list.push(rule);
    } else {
      byModule.set(rule.module, [rule]);
      order.push(rule.module);
    }
  }
  return order.map((moduleName) => ({
    id: `templates.${moduleName}`,
    title: `${moduleName.toUpperCase()}: ADMIN-ONLY TEMPLATES`,
    functions: (byModule.get(moduleName) ?? []).map((rule) => ({
      id: `template.${moduleName}.${rule.pattern}`,
      label: rule.label,
      grants: { viewer: false, developer: false, admin: true },
      tooltip: rule.denial,
      requiredRole: 'admin' as const,
    })),
  }));
}

function summaryFor(resolved: {
  role: PortalRole;
  keycloakGroup: string;
  defaulted: boolean;
}): string {
  if (resolved.defaulted) {
    return 'No Keycloak group matched viewers, developers, or administrators, so this mock assigns Developer. Admin includes everything a Developer can do.';
  }
  return `Your role comes from your Keycloak group: ${resolved.keycloakGroup}. Admin includes everything a Developer can do.`;
}

/** Mock payload. `roleOverride` is honored only by the mock (header or dev server). */
export function buildPortalAccess(
  identity: PortalIdentity,
  roleOverride?: string | null
): PortalAccess {
  const resolved = resolveRole(identity.groups, roleOverride);
  const platform: PortalSection = {
    id: 'platform',
    title: 'PLATFORM',
    functions: [
      {
        id: 'platform.view',
        label: 'View modules, workflows, logs, artifacts',
        grants: { viewer: true, developer: true, admin: true },
      },
      {
        id: 'platform.modules',
        label: 'Enable, disable, pin or reset modules',
        grants: { viewer: false, developer: false, admin: true },
        denial: 'Enabling a module creates cloud resources that everyone shares.',
        requiredRole: 'admin',
      },
      {
        id: 'platform.settings',
        label: 'Portal settings',
        grants: { viewer: false, developer: false, admin: true },
        denial: 'Portal settings change what every signed-in user sees.',
        requiredRole: 'admin',
      },
      {
        id: 'platform.adminApplications',
        label: 'Open Admin applications',
        grants: { viewer: false, developer: false, admin: true },
        denial: 'Admin applications manage the whole platform.',
        requiredRole: 'admin',
      },
      {
        id: 'platform.openArgoWorkflows',
        label: 'Open Argo Workflows',
        grants: { viewer: true, developer: true, admin: true },
      },
    ],
  };
  const workflows: PortalSection = {
    id: 'workflows',
    title: 'WORKFLOWS',
    functions: [
      {
        id: 'workflows.submit',
        label: 'Submit workflow templates',
        grants: { viewer: false, developer: true, admin: true },
        denial: 'Submitting a workflow uses shared build and test capacity.',
        requiredRole: 'developer',
      },
      {
        id: 'workflows.abortOwn',
        label: 'Abort workflows you started',
        grants: { viewer: false, developer: true, admin: true },
        denial: 'Aborting a workflow stops a shared run.',
        requiredRole: 'developer',
      },
      {
        id: 'workflows.abortOthers',
        label: "Abort others' or ownerless workflows",
        grants: { viewer: false, developer: false, admin: true },
        denial: "Aborting someone else's workflow stops their run for everyone.",
        requiredRole: 'admin',
      },
      {
        id: 'workflows.delete',
        label: 'Delete workflow records',
        grants: { viewer: false, developer: false, admin: true },
        tooltip:
          'Admin only. Deleting removes the workflow record and its logs for everyone, including workflows you started.',
        requiredRole: 'admin',
      },
    ],
  };
  return {
    mock: true,
    subject: identity.subject,
    email: identity.email || identity.subject,
    role: resolved.role,
    roleLabel: ROLE_LABEL[resolved.role],
    keycloakGroup: resolved.keycloakGroup,
    summary: summaryFor(resolved),
    moreAccess: MORE_ACCESS,
    roles: PORTAL_ROLES,
    sections: [platform, workflows, ...templateSections(ADMIN_ONLY_TEMPLATES)],
    applications: APPLICATIONS,
    adminOnlyTemplates: ADMIN_ONLY_TEMPLATES,
  };
}

export function functionById(access: PortalAccess, id: string): PortalFunction | undefined {
  for (const section of access.sections) {
    const found = section.functions.find((fn) => fn.id === id);
    if (found) {
      return found;
    }
  }
  return undefined;
}

export function hasGrant(access: PortalAccess | null, id: string): boolean {
  if (!access) {
    return false;
  }
  const fn = functionById(access, id);
  return fn?.grants[access.role] === true;
}

function requiredRoleOf(fn: PortalFunction): PortalRole {
  if (fn.requiredRole) {
    return fn.requiredRole;
  }
  if (fn.grants.viewer) {
    return 'viewer';
  }
  if (fn.grants.developer) {
    return 'developer';
  }
  return 'admin';
}

export function requiredRoleLabel(access: PortalAccess, functionId: string): string {
  const fn = functionById(access, functionId);
  return ROLE_LABEL[fn ? requiredRoleOf(fn) : 'admin'];
}

/** Tooltip for a denied function. Empty when the signed-in role is granted. */
export function gatedTooltip(access: PortalAccess, functionId: string): string {
  const fn = functionById(access, functionId);
  if (!fn || fn.grants[access.role]) {
    return '';
  }
  if (fn.tooltip) {
    return fn.tooltip;
  }
  const required = requiredRoleOf(fn);
  const prefix = required === 'developer' ? 'Developer only.' : 'Admin only.';
  const group =
    required === 'developer' ? 'developers or administrators group' : 'administrators group';
  const reason = fn.denial ?? 'This action is not included in your role.';
  return `${prefix} ${reason} You are signed in as ${access.roleLabel}. Ask a Horizon administrator to add you to the ${group}.`;
}

export function globMatch(pattern: string, name: string): boolean {
  const source = pattern.trim();
  const target = name.trim();
  if (!source || !target) {
    return false;
  }
  let expression = '^';
  for (const char of source) {
    if (char === '*') {
      expression += '.*';
    } else if ('.+?^${}()|[]\\'.includes(char)) {
      expression += `\\${char}`;
    } else {
      expression += char;
    }
  }
  expression += '$';
  return new RegExp(expression, 'i').test(target);
}

export function findAdminTemplate(
  access: PortalAccess | null,
  moduleName: string,
  templateName: string
): AdminOnlyTemplate | undefined {
  if (!access) {
    return undefined;
  }
  const moduleKey = moduleName.trim().toLowerCase();
  return access.adminOnlyTemplates.find((rule) => {
    if (rule.module.trim().toLowerCase() !== moduleKey) {
      return false;
    }
    return globMatch(rule.pattern, templateName);
  });
}

export function applicationAudience(
  access: PortalAccess | null,
  id: string,
  fallback: 'developer' | 'admin'
): 'developer' | 'admin' {
  return access?.applications.find((app) => app.id === id)?.audience ?? fallback;
}

export function applicationAllowed(
  access: PortalAccess | null,
  id: string,
  audience: 'developer' | 'admin'
): boolean {
  if (!access) {
    return false;
  }
  if (audience === 'admin') {
    return hasGrant(access, 'platform.adminApplications');
  }
  if (id === 'argo-workflows') {
    return hasGrant(access, 'platform.openArgoWorkflows');
  }
  return hasGrant(access, 'platform.view');
}

function sameIdentity(left: string, right: string): boolean {
  const a = left.trim().toLowerCase();
  const b = right.trim().toLowerCase();
  return a.length > 0 && a === b;
}

export function isOwnWorkflow(access: PortalAccess, startedBy: string | undefined): boolean {
  const who = (startedBy ?? '').trim();
  if (!who) {
    return false;
  }
  return sameIdentity(who, access.subject) || sameIdentity(who, access.email);
}

export function abortDecision(
  access: PortalAccess | null,
  startedBy: string | undefined
): { allowed: boolean; tooltip: string; badge?: string } {
  if (!access) {
    return { allowed: false, tooltip: 'Checking your access…' };
  }
  const own = isOwnWorkflow(access, startedBy);
  if (
    hasGrant(access, 'workflows.abortOthers') ||
    (own && hasGrant(access, 'workflows.abortOwn'))
  ) {
    return {
      allowed: true,
      tooltip: own
        ? 'You started this workflow, so you can abort it. Request graceful stop.'
        : 'You can abort this workflow. Request graceful stop.',
    };
  }
  if (!own) {
    const who = (startedBy ?? '').trim();
    const whoText = who ? `This one was started by ${who}.` : 'This one has no recorded owner.';
    return {
      allowed: false,
      badge: 'Admin',
      tooltip: `You can only abort workflows you started. ${whoText} Admins can abort any workflow.`,
    };
  }
  return {
    allowed: false,
    badge: requiredRoleLabel(access, 'workflows.abortOwn'),
    tooltip: gatedTooltip(access, 'workflows.abortOwn'),
  };
}
