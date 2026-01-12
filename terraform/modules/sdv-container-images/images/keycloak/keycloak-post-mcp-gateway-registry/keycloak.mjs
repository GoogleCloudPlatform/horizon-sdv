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

import fs from 'fs/promises';
import _ from 'lodash';
import KcAdminClient from '@keycloak/keycloak-admin-client';
import retry from 'async-retry';

const webClientDumpFileName = 'client-mcp-gateway-registry-web.json';
const cliClientDumpFileName = 'client-mcp-gateway-registry-cli.json';

const config = {
  keycloak: {
    baseUrl: process.env.PLATFORM_URL + '/auth',
    username: process.env.KEYCLOAK_USERNAME,
    password: process.env.KEYCLOAK_PASSWORD,
    realm: {
      realm: 'horizon'
    },
    clients: {
      // Client for the Web UI (uses standard browser flow)
      webClient: {
        clientId: 'mcp-gateway-registry-web',
        adminUrl: process.env.MCP_SUBDOMAIN,
        redirectUris: [process.env.MCP_SUBDOMAIN + '/*'],
        protocol: 'openid-connect',
        publicClient: false,
        standardFlowEnabled: true,
        directAccessGrantsEnabled: true,
      },
      // Client for the CLI (uses device flow)
      cliClient: {
        clientId: 'mcp-gateway-registry-cli',
        protocol: 'openid-connect',
        publicClient: true,
        standardFlowEnabled: false,
        directAccessGrantsEnabled: false,
        attributes: {
          'oauth2.device.authorization.grant.enabled': true
        }
      },
    },
    mappers: [
      {
        name: 'groups',
        protocol: 'openid-connect',
        protocolMapper: 'oidc-group-membership-mapper',
        consentRequired: false,
        config: {
          'full.path': 'false',
          'id.token.claim': 'true',
          'access.token.claim': 'true',
          'claim.name': 'groups',
          'userinfo.token.claim': 'true'
        }
      }
    ],
    adminUser: {
      username: process.env.MCP_GATEWAY_REGISTRY_ADMIN_USERNAME,
      password: process.env.MCP_GATEWAY_REGISTRY_ADMIN_PASSWORD,
      firstName: 'MCP Gateway Registry',
      lastName: 'MCP Gateway Registry',
      email: 'mcp-gateway-registry@mcp-gateway-registry'
    },
    realmGroups: [
      'horizon-mcp-gateway-registry-admins', // admin group
      'horizon-mcp-gateway-registry-users'
    ]
  }
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

// returns the created or updated client
async function createOrUpdateClientIfRequired(clientConfig) {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: clientConfig.clientId});
    if (client) {
      console.info('updating %s client', clientConfig.clientId);
      const targetClient = _.merge({}, client, clientConfig);
      await keycloakAdmin.clients.update({id: client.id, realm: config.keycloak.realm.realm}, targetClient);
    } else {
      console.info('creating %s client', clientConfig.clientId);
      await keycloakAdmin.clients.create(clientConfig);
    }
    clients = await keycloakAdmin.clients.find();
    return _.find(clients, {clientId: clientConfig.clientId});
  } catch (err) {
    throw err;
  }
}

// creates or updates both clients
async function createOrUpdateClients()  {
  try {
    config.keycloak.clients.webClient = await createOrUpdateClientIfRequired(config.keycloak.clients.webClient);
    config.keycloak.clients.cliClient = await createOrUpdateClientIfRequired(config.keycloak.clients.cliClient);
  } catch (err) {
    throw err
  }
}

async function createAdminUserIfRequired()  {
  try {
    let users = await keycloakAdmin.users.find();
    let user = _.find(users, {username: config.keycloak.adminUser.username});

    if (user) {
      console.info('deleting old instance of %s user', config.keycloak.adminUser.username);
      await keycloakAdmin.users.del({id: user.id});
    }

    console.info('creating %s user', config.keycloak.adminUser.username);
    const new_user = await keycloakAdmin.users.create({
      username: config.keycloak.adminUser.username,
      enabled: true,
      requiredActions: [],
      realm: config.keycloak.realm.realm,
      firstName: config.keycloak.adminUser.firstName,
      lastName: config.keycloak.adminUser.lastName,
      email: config.keycloak.adminUser.email
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

async function addAdminUsertoAdminGroupIfRequired(groupName) {
  try {
    const users = await keycloakAdmin.users.find({ username: config.keycloak.adminUser.username });
    const user = users[0];

    if (!user) {
      console.error(`user "${config.keycloak.adminUser.username}" does not exist.`);
      return;
    }

    const groups = await keycloakAdmin.groups.find({ search: groupName });
    const group = groups[0];

    if (!group) {
      console.error(`group "${groupName}" does not exist.`);
      return;
    }

    const userGroups = await keycloakAdmin.users.listGroups({ id: user.id });
    const isGroupAssigned = userGroups.some(g => g.id === group.id);

    if (isGroupAssigned) {
      console.info(`user "${config.keycloak.adminUser.username}" already in group "${groupName}".`);
    } else {
      console.log(`adding user "${config.keycloak.adminUser.username}" to group "${groupName}".`);
      await keycloakAdmin.users.addToGroup({ id: user.id, groupId: group.id });
    }
  } catch (err) {
    throw err;
  }
}

async function generateClientSecretFiles(clientId, fileName)  {
  try {
    const client = (await keycloakAdmin.clients.find({ clientId }))[0];
    if (client) {
      console.info('dumping %s client data into json file', clientId);
      fs.writeFile(fileName, JSON.stringify(client));
    } else {
      console.error('client %s not found, cannot dump secret file', clientId);
    }
  } catch (err) {
    throw err
  }
}

// generates secret files for both clients
async function generateSecretFiles()  {
  try {
    await generateClientSecretFiles(config.keycloak.clients.webClient.clientId, webClientDumpFileName);
    await generateClientSecretFiles(config.keycloak.clients.cliClient.clientId, cliClientDumpFileName);
  } catch (err) {
    throw err;
  }
}

async function createRealmGroupsIfRequired() {
  const realmGroupNames = config.keycloak.realmGroups || [];

  for (const realmGroupName of realmGroupNames) {
    try {
      const existingGroups = await keycloakAdmin.groups.find({ search: realmGroupName });
      const matchedGroup = existingGroups.find(group => group.name === realmGroupName);

      if (matchedGroup) {
        console.info(`group "${realmGroupName}" already exists.`);
      } else {
        console.log(`creating group "${realmGroupName}".`);
        await keycloakAdmin.groups.create({ name: realmGroupName });
      }
    } catch (err) {
      throw err;
    }
  }
}

async function createProtocolMappersForClientIfRequired(clientId) {
  try {
    const allClients = await keycloakAdmin.clients.find();

    const targetClient = allClients.find(c => c.clientId === clientId);
    if (!targetClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    // Fetch mappers for client
    const existingMappers = await keycloakAdmin.clients.listProtocolMappers({ id: targetClient.id });
    for (const mapper of config.keycloak.mappers) {
      const mapperExists = existingMappers.some(m => m.name === mapper.name && m.protocolMapper === mapper.protocolMapper);

      if (mapperExists) {
        console.info(`${mapper.name} mapper already exists for "${clientId}" client.`);
      } else {
        console.log(`adding ${mapper.name} mapper to "${clientId}" client.`);
        await keycloakAdmin.clients.addProtocolMapper({ id: targetClient.id }, mapper);
      }
    }
  } catch (err) {
    throw err;
  }
}

// creates protocol mappers for both clients
async function createProtocolMappersForClients() {
  await createProtocolMappersForClientIfRequired(config.keycloak.clients.webClient.clientId);
  await createProtocolMappersForClientIfRequired(config.keycloak.clients.cliClient.clientId);
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await getRealm();
    await createOrUpdateClients();
    await createAdminUserIfRequired();
    await addAdminUsertoAdminGroupIfRequired(config.keycloak.realmGroups[0]);
    await generateSecretFiles();
    await createRealmGroupsIfRequired();
    await createProtocolMappersForClients();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });
