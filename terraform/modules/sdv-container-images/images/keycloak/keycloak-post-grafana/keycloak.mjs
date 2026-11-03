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

const config = {
  keycloak: {
    baseUrl: process.env.PLATFORM_URL + '/auth',
    username: process.env.KEYCLOAK_USERNAME,
    password: process.env.KEYCLOAK_PASSWORD,
    realm: {
      realm: 'horizon'
    },
    client: {
      clientId: 'grafana-oauth',
      adminUrl: process.env.DOMAIN + '/grafana',
      redirectUris: [process.env.DOMAIN + '/grafana/login/generic_oauth/*'],
      protocol: 'openid-connect',
      publicClient: false
    },
    adminUser: {
      username: process.env.GRAFANA_ADMIN_USERNAME,
      password: process.env.GRAFANA_ADMIN_PASSWORD,
      firstName: 'Grafana',
      lastName: 'Grafana',
      email: 'grafana@grafana'
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
      await keycloakAdmin.clients.update({id: client.id, realm: config.keycloak.realm.realm}, _.merge(client, config.keycloak.client));
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
      fs.writeFile('client-grafana.json', JSON.stringify(client));
    }
  } catch (err) {
    throw err
  }
}

async function DisableFullScopeIfRequired() {
  const clientId = config.keycloak.client.clientId;

  try {
    const clients = await keycloakAdmin.clients.find();
    const grafanaClient = clients.find(client => client.clientId === clientId);

    if (!grafanaClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    if (grafanaClient.fullScopeAllowed === false) {
      console.info(`"Full scope allowed" is already disabled for client "${clientId}".`);
    } else {
      console.log(`disabling "Full scope allowed" for client "${clientId}".`);
      await keycloakAdmin.clients.update(
        { id: grafanaClient.id, realm: config.keycloak.realm.realm },
        { ...grafanaClient, fullScopeAllowed: false }
      );
    }
  } catch (err) {
    throw err;
  }
}

async function createGrafanaClientRolesIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const grafanaClient = clients.find(client => client.clientId === clientId);

    if (!grafanaClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const existingRoles = await keycloakAdmin.clients.listRoles({ id: grafanaClient.id });

    for (const roleName of clientRoleNames) {
      const roleExists = existingRoles.some(role => role.name === roleName);
      if (roleExists) {
        console.info(`client role "${roleName}" already exists for "${clientId}".`);
        continue;
      }

      await keycloakAdmin.clients.createRole({id: grafanaClient.id, name: roleName});
      console.log(`client role "${roleName}" created for client "${clientId}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function mapGrafanaClientRolesToGroupsIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const grafanaClient = clients.find(client => client.clientId === clientId);

    if (!grafanaClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const allGroups = await keycloakAdmin.groups.find();

    for (const clientRoleName of clientRoleNames) {
      const clientRole = await keycloakAdmin.clients.findRole({id: grafanaClient.id, roleName: clientRoleName});

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

        const mappedRoles = await keycloakAdmin.groups.listClientRoleMappings({id: group.id, clientUniqueId: grafanaClient.id});
        const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

        if (alreadyMapped) {
          console.info(`client role "${clientRoleName}" is already mapped to group "${groupName}".`);
          continue;
        }

        await keycloakAdmin.groups.addClientRoleMappings({
          id: group.id,
          clientUniqueId: grafanaClient.id,
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
    const grafanaClient = clients.find(c => c.clientId === clientId);

    if (!grafanaClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    for (const clientRoleName of clientRoleNamesToAssign) {
      const clientRole = await keycloakAdmin.clients.findRole({
        id: grafanaClient.id,
        roleName: clientRoleName
      });

      if (!clientRole) {
        console.warn(`client role "${clientRoleName}" does not exist for "${clientId}".`);
        continue;
      }

      const mappedRoles = await keycloakAdmin.users.listClientRoleMappings({
        id: user.id,
        clientUniqueId: grafanaClient.id
      });
      const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

      if (alreadyMapped) {
        console.info(`user "${user.username}" already has client role "${clientRoleName}" on "${clientId}".`);
        continue;
      }

      await keycloakAdmin.users.addClientRoleMappings({
        id: user.id,
        clientUniqueId: grafanaClient.id,
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
    await createGrafanaClientRolesIfRequired();
    await mapGrafanaClientRolesToGroupsIfRequired();
    await mapUsersToClientRoleIfRequired();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });