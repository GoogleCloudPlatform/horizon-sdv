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

import fs from 'fs/promises';
import _ from 'lodash';
import KcAdminClient from '@keycloak/keycloak-admin-client';
import retry from 'async-retry';

const argocdRedirectUris = () => {
  const d = process.env.DOMAIN;
  return [
    `${d}/argocd/auth/callback`,
    `${d}/argocd/api/dex/callback`,
    `${d}/argocd/pkce/verify`,
    `${d}/argocd/*`,
    // If argocd-cm url is missing the /argocd suffix, Dex/OIDC uses host-root paths.
    `${d}/api/dex/callback`,
    // Workaround when rootPath is omitted from generated OAuth URLs (argoproj/argo-cd#26233, #18045).
    `${d}/auth/callback`,
    `${d}/pkce/verify`,
    // Argo CD CLI: `argocd login <host> --sso` (default --sso-port 8085).
    'http://127.0.0.1:8085/auth/callback',
    'http://localhost:8085/auth/callback',
    'http://127.0.0.1:8085/pkce/verify',
    'http://localhost:8085/pkce/verify',
  ];
};

const config = {
  keycloak: {
    baseUrl: process.env.PLATFORM_URL + '/auth',
    username: process.env.KEYCLOAK_USERNAME,
    password: process.env.KEYCLOAK_PASSWORD,
    realm: {
      realm: 'horizon'
    },
    client: {
      clientId: 'argocd',
      adminUrl: process.env.DOMAIN + '/argocd',
      redirectUris: argocdRedirectUris(),
      webOrigins: [process.env.DOMAIN, `${process.env.DOMAIN}/argocd`],
      protocol: 'openid-connect',
      publicClient: false
    },
    adminUser: {
      username: process.env.ARGOCD_ADMIN_USERNAME,
      password: process.env.ARGOCD_ADMIN_PASSWORD,
      firstName: 'Argocd',
      lastName: 'Argocd',
      email: 'argocd@argocd'
    },
    ClientRoles: [
      'administrators',
      'viewers'
    ]
  }
};

const clientRoleToGroup = {
  administrators: ['administrators'],
  viewers: ['viewers']
};

// Argo CD uses `oidc.groupsClaim: roles` (see configure.sh). Keycloak does not put client
// roles in a top-level `roles` JWT claim by default — add an explicit mapper.
const ARGOCD_ROLES_CLAIM_MAPPER = 'argocd-rbac-client-roles-claim';

const keycloakAdmin = new KcAdminClient({
  baseUrl: config.keycloak.baseUrl
});

async function waitForKeycloak() {
  const opts = {
    retries: 100,
    minTimeout: 2000,
    factor: 1,
    onRetry: (err) => {console.info(`waiting for ${config.keycloak.baseUrl}...`, err.message)}
  };
  await retry(login, opts);
}

async function login()  {
  try {
    await keycloakAdmin.auth({
      'username': config.keycloak.username,
      'password': config.keycloak.password,
      'grantType': 'password',
      'clientId': 'admin-cli'
    });
  } catch (err) {
    throw err
  }
}

async function getRealm()  {
  try {
    let realm = await keycloakAdmin.realms.findOne({
      realm: config.keycloak.realm.realm,
    });
    keycloakAdmin.setConfig({
      realmName: realm.realm,
    });
    realm.keys = await keycloakAdmin.realms.getKeys({realm: realm.realm});
    config.keycloak.realm = realm;
  } catch (err) {
    throw err
  }
}

async function createClientIfRequired()  {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});
    if (client) {
      console.info('updating %s client', config.keycloak.client.clientId);
      const merged = _.merge({}, client, config.keycloak.client);
      merged.redirectUris = argocdRedirectUris();
      merged.webOrigins = [process.env.DOMAIN, `${process.env.DOMAIN}/argocd`];
      await keycloakAdmin.clients.update({id: client.id, realm: config.keycloak.realm.realm}, merged);
    } else {
      console.info('creating %s client', config.keycloak.client.clientId);
      await keycloakAdmin.clients.create(config.keycloak.client);
    }
    clients = await keycloakAdmin.clients.find();
    client = _.find(clients, {clientId: config.keycloak.client.clientId});
    config.keycloak.client = client;
  } catch (err) {
    throw err
  }
}

async function createUserIfRequired()  {
  try {
    let users = await keycloakAdmin.users.find();
    let user = _.find(users, {username: config.keycloak.adminUser.username});

    if (user) {
      console.info('user "%s" already exists, skipping create and password reset', config.keycloak.adminUser.username);
      return;
    }

    console.info('creating %s user', config.keycloak.adminUser.username);
    const new_user = await keycloakAdmin.users.create({
      username: config.keycloak.adminUser.username,
      enabled: true,
      requiredActions: [],
      realm: config.keycloak.realm.realm,
      firstName: config.keycloak.adminUser.firstName,
      lastName: config.keycloak.adminUser.lastName,
      email: config.keycloak.adminUser.email,
      emailVerified: true
    });

    await keycloakAdmin.users.resetPassword({
      id: new_user.id,
      realm: config.keycloak.realm.realm,
      credential: {temporary: false, type: 'password', value: config.keycloak.adminUser.password}
    });

  } catch (err) {
    throw err
  }
}

async function generateSecretFiles()  {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});

    if (client) {
      console.info('dumping %s client data into json file', config.keycloak.client.clientId);
      fs.writeFile('client-argocd.json', JSON.stringify(client));
    }
  } catch (err) {
    throw err
  }
}

async function DisableFullScopeIfRequired() {
  const clientId = config.keycloak.client.clientId;

  try {
    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(client => client.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    if (argocdClient.fullScopeAllowed === false) {
      console.info(`"Full scope allowed" is already disabled for client "${clientId}".`);
    } else {
      console.log(`disabling "Full scope allowed" for client "${clientId}".`);
      await keycloakAdmin.clients.update(
        { id: argocdClient.id, realm: config.keycloak.realm.realm },
        { ...argocdClient, fullScopeAllowed: false }
      );
    }
  } catch (err) {
    throw err;
  }
}

async function createArgocdClientRolesIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(client => client.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const existingRoles = await keycloakAdmin.clients.listRoles({ id: argocdClient.id });

    for (const roleName of clientRoleNames) {
      const roleExists = existingRoles.some(role => role.name === roleName);
      if (roleExists) {
        console.info(`client role "${roleName}" already exists for "${clientId}".`);
        continue;
      }

      await keycloakAdmin.clients.createRole({id: argocdClient.id, name: roleName});
      console.log(`client role "${roleName}" created for client "${clientId}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function ensureArgocdRolesJwtClaimMapper() {
  const clientId = config.keycloak.client.clientId;

  try {
    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(c => c.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist; skip roles JWT mapper.`);
      return;
    }

    const mapperPayload = {
      name: ARGOCD_ROLES_CLAIM_MAPPER,
      protocol: 'openid-connect',
      protocolMapper: 'oidc-usermodel-client-role-mapper',
      consentRequired: false,
      config: {
        multivalued: 'true',
        'userinfo.token.claim': 'true',
        'id.token.claim': 'true',
        'access.token.claim': 'true',
        'claim.name': 'roles',
        'jsonType.label': 'String',
        'usermodel.clientRoleMapping.clientId': clientId
      }
    };

    const mappers = await keycloakAdmin.clients.listProtocolMappers({ id: argocdClient.id });
    const existing = mappers.find(m => m.name === ARGOCD_ROLES_CLAIM_MAPPER);

    if (existing) {
      if (existing.protocolMapper !== mapperPayload.protocolMapper) {
        await keycloakAdmin.clients.delProtocolMapper({
          id: argocdClient.id,
          mapperId: existing.id
        });
        await keycloakAdmin.clients.addProtocolMapper({ id: argocdClient.id }, mapperPayload);
        console.log('replaced protocol mapper %s on client %s', ARGOCD_ROLES_CLAIM_MAPPER, clientId);
        return;
      }
      await keycloakAdmin.clients.updateProtocolMapper(
        { id: argocdClient.id, mapperId: existing.id },
        { ...mapperPayload, id: existing.id }
      );
      console.info('updated protocol mapper %s on client %s', ARGOCD_ROLES_CLAIM_MAPPER, clientId);
      return;
    }

    await keycloakAdmin.clients.addProtocolMapper({ id: argocdClient.id }, mapperPayload);
    console.log('created protocol mapper %s on client %s', ARGOCD_ROLES_CLAIM_MAPPER, clientId);
  } catch (err) {
    throw err;
  }
}

async function mapArgocdClientRolesToGroupsIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(client => client.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const allGroups = await keycloakAdmin.groups.find();

    for (const clientRoleName of clientRoleNames) {
      const clientRole = await keycloakAdmin.clients.findRole({id: argocdClient.id, roleName: clientRoleName});

      if (!clientRole) {
        console.warn(`client role "${clientRoleName}" does not exist in "${clientId}".`);
        continue;
      }

      const groupNames = clientRoleToGroup[clientRoleName];
      if (!groupNames) {
        console.warn(`no group mapping defined for client role "${clientRoleName}".`);
        continue;
      }

      const names = Array.isArray(groupNames) ? groupNames : [groupNames];
      for (const groupName of names) {
        const group = allGroups.find(g => g.name === groupName);

        if (!group) {
          console.warn(`group "${groupName}" does not exist.`);
          continue;
        }

        const mappedRoles = await keycloakAdmin.groups.listClientRoleMappings({id: group.id, clientUniqueId: argocdClient.id});
        const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

        if (alreadyMapped) {
          console.info(`client role "${clientRoleName}" is already mapped to group "${groupName}".`);
          continue;
        }

        await keycloakAdmin.groups.addClientRoleMappings({
          id: group.id,
          clientUniqueId: argocdClient.id,
          roles: [{
            id: clientRole.id,
            name: clientRole.name
          }]
        });
        console.log(`client role "${clientRoleName}" mapped to group "${groupName}".`);
      }
    }
  } catch (err) {
    throw err;
  }
}

async function mapUsersToClientRoleIfRequired() {
  const searchGroup = 'admin';
  const clientId = config.keycloak.client.clientId;

  const clientRoleNamesToAssign = [];
  for (const [roleName, mappedGroupNames] of Object.entries(clientRoleToGroup)) {
    const names = Array.isArray(mappedGroupNames) ? mappedGroupNames : [mappedGroupNames];
    if (names.some(gn => gn.includes(searchGroup))) {
      clientRoleNamesToAssign.push(roleName);
    }
  }

  try {
    const users = await keycloakAdmin.users.find();
    const user = _.find(users, { username: config.keycloak.adminUser.username });

    if (!user) {
      console.error(`user "${config.keycloak.adminUser.username}" does not exist.`);
      return;
    }

    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(c => c.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    for (const clientRoleName of clientRoleNamesToAssign) {
      const clientRole = await keycloakAdmin.clients.findRole({
        id: argocdClient.id,
        roleName: clientRoleName
      });

      if (!clientRole) {
        console.warn(`client role "${clientRoleName}" does not exist for "${clientId}".`);
        continue;
      }

      const mappedRoles = await keycloakAdmin.users.listClientRoleMappings({
        id: user.id,
        clientUniqueId: argocdClient.id
      });
      const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

      if (alreadyMapped) {
        console.info(`user "${user.username}" already has client role "${clientRoleName}" on "${clientId}".`);
        continue;
      }

      await keycloakAdmin.users.addClientRoleMappings({
        id: user.id,
        clientUniqueId: argocdClient.id,
        roles: [{ id: clientRole.id, name: clientRole.name }]
      });
      console.log(`user "${user.username}" assigned client role "${clientRoleName}" on "${clientId}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await getRealm();
    await createClientIfRequired();
    await createUserIfRequired();
    await generateSecretFiles();
    await DisableFullScopeIfRequired();
    await createArgocdClientRolesIfRequired();
    await ensureArgocdRolesJwtClaimMapper();
    await mapArgocdClientRolesToGroupsIfRequired();
    await mapUsersToClientRoleIfRequired();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });