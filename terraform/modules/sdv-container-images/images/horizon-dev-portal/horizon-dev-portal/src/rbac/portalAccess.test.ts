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
import { describe, expect, it } from 'vitest';
import {
  abortDecision,
  applicationAllowed,
  applicationAudience,
  buildPortalAccess,
  findAdminTemplate,
  functionById,
  gatedTooltip,
  globMatch,
  hasGrant,
  identityFromClaims,
  identityFromToken,
  isOwnWorkflow,
  requiredRoleLabel,
  resolveRole,
} from './portalAccess';
import type { PortalAccess, PortalIdentity, PortalRole } from './portalAccess';

const EMAIL = 'some.user@accenture.com';

function identity(groups: string[], subject = EMAIL): PortalIdentity {
  return { subject, email: subject, groups };
}

function accessFor(role: PortalRole, subject = EMAIL): PortalAccess {
  const group = role === 'admin' ? 'administrators' : role === 'viewer' ? 'viewers' : 'developers';
  return buildPortalAccess(identity([group], subject));
}

function token(payload: Record<string, unknown>): string {
  const body = btoa(JSON.stringify(payload))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `header.${body}.sig`;
}

describe('resolveRole', () => {
  it('keeps the highest matching group', () => {
    expect(resolveRole(['/horizon-developers', 'viewers'])).toEqual({
      role: 'developer',
      keycloakGroup: 'developers',
      defaulted: false,
    });
    expect(resolveRole(['developers', 'administrators'])).toEqual({
      role: 'admin',
      keycloakGroup: 'administrators',
      defaulted: false,
    });
    expect(resolveRole(['admins', 'horizon-viewers'])).toEqual({
      role: 'admin',
      keycloakGroup: 'administrators',
      defaulted: false,
    });
  });

  it('accepts the mock role override before groups', () => {
    expect(resolveRole(['administrators'], 'viewer')).toEqual({
      role: 'viewer',
      keycloakGroup: 'viewers',
      defaulted: false,
    });
    expect(resolveRole([], ' ADMIN ')).toEqual({
      role: 'admin',
      keycloakGroup: 'administrators',
      defaulted: false,
    });
  });

  it('assigns Developer when nothing matches', () => {
    expect(resolveRole([], 'nope')).toEqual({
      role: 'developer',
      keycloakGroup: 'developers',
      defaulted: true,
    });
    expect(resolveRole(['realm-admin', 'offline_access'])).toMatchObject({
      role: 'developer',
      defaulted: true,
    });
  });
});

describe('identity', () => {
  it('reads groups and realm roles, and ignores a top-level roles array', () => {
    const parsed = identityFromClaims({
      email: EMAIL,
      preferred_username: EMAIL,
      groups: '/org/horizon-developers',
      realm_access: { roles: ['viewers'] },
      roles: ['administrators'],
    });
    expect(parsed).toEqual({
      subject: EMAIL,
      email: EMAIL,
      groups: ['/org/horizon-developers', 'viewers'],
    });
    expect(resolveRole(parsed.groups).role).toBe('developer');
  });

  it('falls back through name and subject when email is missing', () => {
    expect(identityFromClaims({ name: 'Some User', sub: 'user-1' })).toEqual({
      subject: 'Some User',
      email: 'Some User',
      groups: [],
    });
    expect(identityFromClaims({})).toEqual({ subject: '', email: '', groups: [] });
  });

  it('decodes a bearer payload and rejects a token that is not a JWT', () => {
    const parsed = identityFromToken(
      token({
        preferred_username: EMAIL,
        email: EMAIL,
        groups: ['horizon-administrators'],
      })
    );
    expect(parsed.groups).toEqual(['horizon-administrators']);
    expect(resolveRole(parsed.groups).role).toBe('admin');
    expect(identityFromToken(undefined)).toEqual({ subject: '', email: '', groups: [] });
    expect(identityFromToken('not-a-jwt')).toEqual({ subject: '', email: '', groups: [] });
    expect(identityFromToken('a.%%% .b')).toEqual({ subject: '', email: '', groups: [] });
  });
});

describe('buildPortalAccess', () => {
  it('builds the mock contract for a developer', () => {
    const access = buildPortalAccess(identity(['developers']));
    expect(access.mock).toBe(true);
    expect(access.role).toBe('developer');
    expect(access.roleLabel).toBe('Developer');
    expect(access.email).toBe(EMAIL);
    expect(access.summary).toContain('Keycloak group: developers');
    expect(access.sections.map((section) => section.title)).toEqual([
      'PLATFORM',
      'WORKFLOWS',
      'WORKLOADS-ANDROID: ADMIN-ONLY TEMPLATES',
      'REMOTIVE-TOPOLOGY: ADMIN-ONLY TEMPLATES',
    ]);
    expect(access.applications.find((app) => app.id === 'grafana')?.audience).toBe('admin');
    expect(access.applications.find((app) => app.id === 'argo-workflows')?.audience).toBe(
      'developer'
    );
  });

  it('says when the mock defaulted the role', () => {
    const access = buildPortalAccess(identity([]));
    expect(access.role).toBe('developer');
    expect(access.summary).toContain('this mock assigns Developer');
  });

  it('lets the role override win', () => {
    expect(buildPortalAccess(identity(['viewers']), 'admin').role).toBe('admin');
  });
});

describe('grants', () => {
  const ids = [
    'platform.view',
    'platform.modules',
    'platform.settings',
    'platform.adminApplications',
    'platform.openArgoWorkflows',
    'workflows.submit',
    'workflows.abortOwn',
    'workflows.abortOthers',
    'workflows.delete',
  ];

  it('matches the role matrix', () => {
    const expectGrant = (role: PortalRole, granted: string[]) => {
      const access = accessFor(role);
      for (const id of ids) {
        expect(hasGrant(access, id), `${role} ${id}`).toBe(granted.includes(id));
      }
    };
    expectGrant('viewer', ['platform.view', 'platform.openArgoWorkflows']);
    expectGrant('developer', [
      'platform.view',
      'platform.openArgoWorkflows',
      'workflows.submit',
      'workflows.abortOwn',
    ]);
    expectGrant('admin', ids);
  });

  it('gives Admin every function a Developer has', () => {
    const developer = accessFor('developer');
    const admin = accessFor('admin');
    for (const section of developer.sections) {
      for (const fn of section.functions) {
        if (fn.grants.developer) {
          expect(hasGrant(admin, fn.id), fn.id).toBe(true);
        }
      }
    }
  });

  it('denies every function while access is still loading', () => {
    expect(hasGrant(null, 'platform.view')).toBe(false);
  });

  it('names the role a denied control requires', () => {
    const developer = accessFor('developer');
    expect(requiredRoleLabel(developer, 'workflows.submit')).toBe('Developer');
    expect(requiredRoleLabel(developer, 'workflows.delete')).toBe('Admin');
    expect(requiredRoleLabel(developer, 'missing')).toBe('Admin');
  });

  it('builds the denial tooltip and stays quiet when the role is granted', () => {
    const developer = accessFor('developer');
    const viewer = accessFor('viewer');
    expect(gatedTooltip(developer, 'platform.view')).toBe('');
    expect(gatedTooltip(developer, 'workflows.delete')).toBe(
      'Admin only. Deleting removes the workflow record and its logs for everyone, including workflows you started.'
    );
    expect(gatedTooltip(developer, 'platform.modules')).toBe(
      'Admin only. Enabling a module creates cloud resources that everyone shares. You are signed in as Developer. Ask a Horizon administrator to add you to the administrators group.'
    );
    expect(gatedTooltip(viewer, 'workflows.submit')).toBe(
      'Developer only. Submitting a workflow uses shared build and test capacity. You are signed in as Viewer. Ask a Horizon administrator to add you to the developers or administrators group.'
    );
  });
});

describe('function lookup', () => {
  it('looks up a function by id', () => {
    const developer = accessFor('developer');
    expect(functionById(developer, 'workflows.delete')?.requiredRole).toBe('admin');
    expect(functionById(developer, 'missing.function')).toBeUndefined();
  });
});

describe('admin-only templates', () => {
  const developer = accessFor('developer');

  it('matches a template glob inside its module only', () => {
    expect(globMatch('cf-instance-template-*-delete', 'cf-instance-template-arm64-delete')).toBe(
      true
    );
    expect(globMatch('cf-instance-template-*-delete', 'aaos-builder')).toBe(false);
    expect(globMatch('a.b', 'axb')).toBe(false);
    expect(globMatch('', 'name')).toBe(false);
    expect(
      findAdminTemplate(developer, 'Workloads-Android', 'cf-instance-template-x86-delete')?.pattern
    ).toBe('cf-instance-template-*-delete');
    expect(
      findAdminTemplate(developer, 'other-module', 'cf-instance-template-x86-delete')
    ).toBeUndefined();
    expect(findAdminTemplate(null, 'workloads-android', 'cf-instance-template-x86-delete')).toBe(
      undefined
    );
  });
});

describe('applications', () => {
  it('uses the contract audience and the grant for that audience', () => {
    const developer = accessFor('developer');
    const viewer = accessFor('viewer');
    const admin = accessFor('admin');
    expect(applicationAudience(developer, 'keycloak', 'developer')).toBe('admin');
    expect(applicationAudience(null, 'custom', 'developer')).toBe('developer');
    expect(applicationAllowed(null, 'gerrit', 'developer')).toBe(false);
    expect(applicationAllowed(developer, 'gerrit', 'developer')).toBe(true);
    expect(applicationAllowed(developer, 'keycloak', 'admin')).toBe(false);
    expect(applicationAllowed(developer, 'argo-workflows', 'developer')).toBe(true);
    expect(applicationAllowed(viewer, 'gerrit', 'developer')).toBe(true);
    expect(applicationAllowed(viewer, 'argo-workflows', 'developer')).toBe(true);
    expect(applicationAllowed(admin, 'grafana', 'admin')).toBe(true);
  });
});

describe('abortDecision', () => {
  it('lets a developer abort only a workflow they started', () => {
    const developer = accessFor('developer');
    expect(isOwnWorkflow(developer, EMAIL)).toBe(true);
    expect(isOwnWorkflow(developer, ' Someone@Else ')).toBe(false);
    expect(isOwnWorkflow(developer, '  ')).toBe(false);

    expect(abortDecision(developer, EMAIL)).toEqual({
      allowed: true,
      tooltip: 'You started this workflow, so you can abort it. Request graceful stop.',
    });
    expect(abortDecision(developer, 'other.user@accenture.com')).toEqual({
      allowed: false,
      badge: 'Admin',
      tooltip:
        'You can only abort workflows you started. This one was started by other.user@accenture.com. Admins can abort any workflow.',
    });
    expect(abortDecision(developer, '').tooltip).toContain('no recorded owner');
  });

  it('lets an admin abort someone else’s workflow', () => {
    const admin = accessFor('admin');
    expect(abortDecision(admin, 'other.user@accenture.com')).toEqual({
      allowed: true,
      tooltip: 'You can abort this workflow. Request graceful stop.',
    });
  });

  it('denies a viewer even for their own workflow', () => {
    const viewer = accessFor('viewer');
    const decision = abortDecision(viewer, EMAIL);
    expect(decision.allowed).toBe(false);
    expect(decision.badge).toBe('Developer');
    expect(decision.tooltip).toContain('Developer only.');
  });

  it('waits while access is unknown', () => {
    expect(abortDecision(null, EMAIL)).toEqual({
      allowed: false,
      tooltip: 'Checking your access…',
    });
  });
});
