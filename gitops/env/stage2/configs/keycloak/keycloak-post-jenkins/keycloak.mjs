// Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
      clientId: 'jenkins',
      adminUrl: process.env.DOMAIN + '/jenkins',
      redirectUris: [process.env.DOMAIN + '/jenkins/*'],
      protocol: 'openid-connect',
      publicClient: false
    },
    adminUser: {
      username: process.env.JENKINS_ADMIN_USERNAME,
      password: process.env.JENKINS_ADMIN_PASSWORD,
      firstName: 'Jenkins',
      lastName: 'Jenkins',
      email: 'jenkins@jenkins'
    },
    clientScope:{
      clientScopeName: 'groups'
    },
    rolesAndGroups: [
      'horizon-jenkins-administrators',
      'horizon-jenkins-workloads-developers',
      'horizon-jenkins-workloads-users'
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

async function generateSecretFiles()  {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});

    if (client) {
      console.info('dumping %s client data into json file', config.keycloak.client.clientId);
      fs.writeFile('client-jenkins.json', JSON.stringify(client));
    }

  } catch (err) {
    throw err
  }
}

async function addGroupsClientScopeToJenkinsClientIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientScopeName = config.keycloak.clientScope.clientScopeName;

  try {
    const clients = await keycloakAdmin.clients.find();
    const jenkinsClient = clients.find(client => client.clientId === clientId);

    if (!jenkinsClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const clientScopes = await keycloakAdmin.clientScopes.find();
    const groupsScope = clientScopes.find(scope => scope.name === clientScopeName);
    
    if (!groupsScope) {
      console.error(`client scope "${clientScopeName}" does not exist.`);
      return;
    }

    const defaultScopes = await keycloakAdmin.clients.listDefaultClientScopes({ id: jenkinsClient.id });
    const isGroupsScopeAssigned = defaultScopes.some(scope => scope.id === groupsScope.id);
    
    if (isGroupsScopeAssigned) {
      console.info('"groups" client scope already exists in "jenkins" client.');
    } else {
      console.log('adding "groups" client scope to "jenkins" client.');
      await keycloakAdmin.clients.addDefaultClientScope({id: jenkinsClient.id, clientScopeId: groupsScope.id,});
    }
  } catch (err) {
    throw err;
  }
}

async function createJenkinsRealmRolesIfRequired() {
  const realmRoleNames = config.keycloak.rolesAndGroups;

  for (const realmRoleName of realmRoleNames) {
    try {
      let realmRole = await keycloakAdmin.roles.findOneByName({name: realmRoleName});
      if (realmRole) {
        console.info(`role ${realmRoleName} exists`);
      } else {
        console.log(`creating ${realmRoleName} role`);
        await keycloakAdmin.roles.create({name: realmRoleName});
      }
    } catch (err) {
      throw err;
    }
  }
}

async function createJenkinsClientRolesIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.rolesAndGroups;

  try {
    const clients = await keycloakAdmin.clients.find();
    const jenkinsClient = clients.find(client => client.clientId === clientId);

    if (!jenkinsClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const existingRoles = await keycloakAdmin.clients.listRoles({ id: jenkinsClient.id });

    for (const roleName of clientRoleNames) {
      const roleExists = existingRoles.some(role => role.name === roleName);
      if (roleExists) {
        console.info(`client role "${roleName}" already exists for "${clientId}".`);
        continue;
      }

      await keycloakAdmin.clients.createRole({id: jenkinsClient.id, name: roleName});
      console.log(`client role "${roleName}" created for client "${clientId}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function createJenkinsRealmGroupsIfRequired() {
  const realmGroupNames = config.keycloak.rolesAndGroups;

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

async function mapJenkinsRealmRolesIntoClientRolesIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const roleNames = config.keycloak.rolesAndGroups;

  try {
    const clients = await keycloakAdmin.clients.find();
    const jenkinsClient = clients.find(client => client.clientId === clientId);
    
    if (!jenkinsClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    for (const roleName of roleNames) {
      const clientRole = await keycloakAdmin.clients.findRole({id: jenkinsClient.id, roleName});

      if (!clientRole) {
        console.warn(`client role "${roleName}" does not exist under client "${clientId}".`);
        continue;
      }

      const realmRole = await keycloakAdmin.roles.findOneByName({ name: roleName });
      if (!realmRole) {
        console.warn(`realm role "${roleName}" does not exist.`);
        continue;
      }

      let parentRole = await keycloakAdmin.clients.findRole({id: jenkinsClient.id, roleName: roleName});
      let childRole = await keycloakAdmin.roles.findOneByName({name: roleName});
      await keycloakAdmin.roles.createComposite({roleId: parentRole.id}, [childRole]);
      console.log(`realm role "${roleName}" mapped into client role "${roleName}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function mapJenkinsClientRolesToGroupsIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const roleGroupNames = config.keycloak.rolesAndGroups;

  try {
    const clients = await keycloakAdmin.clients.find();
    const jenkinsClient = clients.find(client => client.clientId === clientId);
    
    if (!jenkinsClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    for (const roleGroupName of roleGroupNames) {
      const clientRole = await keycloakAdmin.clients.findRole({id: jenkinsClient.id, roleName: roleGroupName});

      if (!clientRole) {
        console.warn(`client role "${roleGroupName}" does not exist in "${clientId}".`);
        continue;
      }

      const allGroups = await keycloakAdmin.groups.find();
      const group = allGroups.find(g => g.name === roleGroupName);

      if (!group) {
        console.warn(`group "${roleGroupName}" does not exist.`);
        continue;
      }

      const mappedRoles = await keycloakAdmin.groups.listClientRoleMappings({id: group.id, clientUniqueId: jenkinsClient.id});
      const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

      if (alreadyMapped) {
        console.info(`client role "${roleGroupName}" is already mapped to group "${roleGroupName}".`);
        continue;
      }

      await keycloakAdmin.groups.addClientRoleMappings({
        id: group.id,
        clientUniqueId: jenkinsClient.id,
        roles: [{
          id: clientRole.id,
          name: clientRole.name
        }]
      });
      console.log(`client role "${roleGroupName}" mapped to group "${roleGroupName}".`);
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
    await addGroupsClientScopeToJenkinsClientIfRequired();
    await createJenkinsRealmRolesIfRequired();
    await createJenkinsRealmGroupsIfRequired();
    await createJenkinsClientRolesIfRequired();
    await mapJenkinsRealmRolesIntoClientRolesIfRequired();
    await mapJenkinsClientRolesToGroupsIfRequired();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });
