/*
 #############################################################################
 # Copyright (c) 2024-2025 Accenture, All Rights Reserved.
 #
 # Licensed under the Apache License, Version 2.0 (the "License");
 # you may not use this file except in compliance with the License.
 # You may obtain a copy of the License at
 #
 #         http://www.apache.org/licenses/LICENSE-2.0
 #
 # Unless required by applicable law or agreed to in writing, software
 # distributed under the License is distributed on an "AS IS" BASIS,
 # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 # See the License for the specific language governing permissions and
 # limitations under the License.
 #
 # -----------------------------------------------------
 # http://www.accenture.com
 # -----------------------------------------------------
 # Description:
 #
 # Automates the setup and configuration of an MTK Connect agent and
 # testbench/device creation.
 #
 # 1. It loads environment variables from a .env file using dotenv.
 # 2. It sets up axios with a base URL and authentication credentials
 #    from the environment.
 # 3. It calls:
 #    - configureAgent which:
 #      - Checks if an agent with a specific registration already exists.
 #      - If not, creates a new agent and sets its permissions.
 #    - configureDevices:
 #      - Creates or updates devices associated with the agent.
 #      - Configures device interfaces (e.g., ADB, button, file system,
 #        MJPEG, terminal, touch, and tunnel).
 #
 #############################################################################
 */

'use strict';

require('dotenv').config();

const fs = require('fs');
const BPromise = require('bluebird');
const axios = require('axios');
const _ = require("lodash");

/**
 * Sets up the Axios configuration with a base URL and authentication
 * credentials from environment variables.
 */
const { MTK_CONNECT_DOMAIN, MTK_CONNECT_USERNAME, MTK_CONNECT_PASSWORD, MTK_CONNECT_REGISTRATION, MTK_CONNECT_TESTBENCH, MTK_CONNECT_TESTBENCH_USER, MTK_CONNECT_DEVICES, MTK_CONNECT_HOST_LIST, MTK_CONNECT_LAUNCH_APPLICATION_NAME, MTK_CONNECT_HOST_ONLY, MTK_CONNECT_DEVICE_PREFIX, MTK_CONNECT_HOST_PORT_LIST} = process.env;
const registration = MTK_CONNECT_REGISTRATION || fs.readFileSync('/usr/src/config/registration.name', 'utf-8');

axios.defaults.baseURL = `https://${MTK_CONNECT_DOMAIN}/mtk-connect`;
axios.defaults.auth = {
  username: MTK_CONNECT_USERNAME,
  password: MTK_CONNECT_PASSWORD
};

/**
 * The agent object that is created or retrieved by the code.
 */
let agent;

/**
 * Configures the agent by creating a new one or retrieving an existing one
 * with the registration name.
 */
async function configureAgent() {
  let agentResponse = await axios.get('/api/v1/agents', {params: {q: JSON.stringify({registration: registration})}})
  if (agentResponse.status === 200 && agentResponse.data.data.length === 1) {
    console.log(`agent with registration ${registration} already exists`);
    agent = agentResponse.data.data[0];
  } else {
    console.log(`creating agent using registration ${registration}`);
    agentResponse = await axios.post('/api/v1/agents', {
      name: MTK_CONNECT_TESTBENCH,
      registration: registration
    })
    agent = agentResponse.data.data;
    if (MTK_CONNECT_TESTBENCH_USER == 'everyone') {
      console.log(`Using group permissions.`)
      const data = {
        group: {
          name: 'everyone'
        },
        permission: 'book'
      }
      await axios.put(`/api/v1/agents/${agent.id}/permissions/group`, data)
    } else {
      console.log(`Using user permissions ${MTK_CONNECT_TESTBENCH_USER}.`)
      const data = {
        user: {
          name: MTK_CONNECT_TESTBENCH_USER
        },
        permission: 'book'
      }
      await axios.put(`/api/v1/agents/${agent.id}/permissions/user`, data)
    }
  }
  console.log(`Created agent using registration ${registration}`);
}

/**
 * Configures the devices by creating them if they don't already exist.
 */
async function configureDevices() {
  await BPromise.mapSeries(_.times(MTK_CONNECT_DEVICES), configureDevice)
}

/**
 * Configures a single device by retrieving it or creating a new one.
 * @param {number} i - The index of the device to configure.
 */
async function configureDevice(i) {
  const index = i + 1;
  const q = {
    'agent.registration': registration,
    index: index
  }

  console.log(`device ${index} ... `);

  const agentResponse = await axios.get('/api/v1/devices', {params: {q: JSON.stringify(q)}})
  if (agentResponse.status === 200 && agentResponse.data.data.length === 1) {
    console.log(`device ${index} already exists`);
  } else {
    console.log(`creating device ${index}`);
    await axios.post(`/api/v1/agents/${agent.id}/devices`, {
      name: `${MTK_CONNECT_DEVICE_PREFIX} ${index}`
    });
  }

  const adbPorts = MTK_CONNECT_HOST_PORT_LIST.split(",");
  const adbHosts = MTK_CONNECT_HOST_LIST.split(",");

  console.log(adbPorts);
  console.log(adbHosts);
  console.log(+adbPorts[index - 1]);
  console.log(adbHosts[index - 1]);

  if (MTK_CONNECT_HOST_ONLY == 'false') {
    const data = {
      interface: {
        'adb': {
          'mode': 'tcp',
          'port': +adbPorts[index - 1],
          'host': adbHosts[index - 1]
        },
        'button': {
          'driver': 'adb',
          'skin': 'Default Android'
        },
        'fs': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb',
              'root': '/'
            },
            {
              'name': 'HOST',
              'driver': 'native',
              'root': '/root'
            }
          ]
        },
        'log': {
          'types': [
            {
              'name': 'logcat',
              'driver': 'logcat'
            }
          ]
        },
        'mjpeg': {
          'types': [
            {
              'name': 'screen',
              'driver': 'minicap',
              'scale': 1.0
            }
          ]
        },
        'terminal': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb',
              'icon': 'adb'
            },
            {
              'name': 'HOST',
              'driver': 'spawn',
              'command': 'bash',
              'args': ['-c', 'cd ~/; bash --login', '']
            }
          ]
        },
        'touch': {
          'driver': 'adb',
          'native': true
        },
        'tunnel': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb'
            }
          ]
        }
      }
    }
    await axios.patch(`/api/v1/agents/${agent.id}/devices/${index}`, data);
  } else {
    const data = {
      interface: {
        'fs': {
          'types': [
            {
              'name': 'HOST',
              'driver': 'native',
              'root': '/root'
            }
          ]
        },
        'terminal': {
          'types': [
            {
              'name': 'HOST',
              'driver': 'spawn',
              'command': 'bash',
              'args': ['-c', '[ -d /home/builder ] && { cd /home/builder; su builder; bash --login; } || { cd /home/jenkins; su jenkins; bash --login; }', '']
            }
          ]
        }
      }
    }

    if (MTK_CONNECT_LAUNCH_APPLICATION_NAME) {
      console.log('openbsw app name');
      data.interface.terminal.types[0].args = ['-c', 'cd /home/builder; su builder -c "eval ./posix/${MTK_CONNECT_LAUNCH_APPLICATION_NAME}"', ''];
    }
    await axios.patch(`/api/v1/agents/${agent.id}/devices/${index}`, data);
  }
}

async function main()  {
  try {
    console.log(`configureAgent`);
    await configureAgent();
  } catch (err) {
    if (MTK_CONNECT_TESTBENCH_USER != 'everyone') {
      console.log(`ERROR: Please check you have logged into MTK Connect at least once to avoid access issues.`);
    }
    throw err;
  }

  console.log(`configureDevices`);
  await configureDevices();
}

main()
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
