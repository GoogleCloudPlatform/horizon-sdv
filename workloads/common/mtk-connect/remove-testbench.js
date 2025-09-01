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
 # Removes an MTK Connect agent and testbench based on its registration.
 #
 # 1. It loads environment variables from a .env file using dotenv.
 # 2. It sets up axios with a base URL and authentication credentials
 #    from the environment.
 # 3. removeAgent:
 #    - Sends a GET request to retrieve agents with a specific registration.
 #    - If exactly one agent is found, it sends a DELETE request to remove
 #      that agent.
 #
 #############################################################################
 */

'use strict';

require('dotenv').config();

const fs = require('fs');
const axios = require('axios');
const _ = require("lodash");

/**
 * Sets up the Axios configuration with a base URL and authentication
 * credentials from environment variables.
 */
const { MTK_CONNECT_DOMAIN, MTK_CONNECT_USERNAME, MTK_CONNECT_PASSWORD, MTK_CONNECT_REGISTRATION, MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES, MTK_CONNECT_TESTBENCH } = process.env;

axios.defaults.baseURL = `https://${MTK_CONNECT_DOMAIN}/mtk-connect`;
axios.defaults.auth = {
  username: MTK_CONNECT_USERNAME,
  password: MTK_CONNECT_PASSWORD
};

/**
 * Remove MTK Connect agent based on registration and offline testbenches.
 */
async function removeAgent() {

  if (MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES) {
    let response = await axios.get('/api/v1/agents');

    if (response.status === 200) {
      for (let testbench of response.data.data) {
        if (testbench.online === true) {
          console.log(`${testbench.name}: ${testbench.id} is online, skip.`);
        } else {
          if (testbench.name === MTK_CONNECT_TESTBENCH) {
            console.log(`${testbench.name}: ${testbench.id} is offline, delete.`);
            await axios.delete(`/api/v1/agents/${testbench.id}`);
            break;
          }
        }
      }
    } else {
      console.log(`error ${response.status}`);
    }
  } else {
    const registration = MTK_CONNECT_REGISTRATION || fs.readFileSync('/usr/src/config/registration.name', 'utf-8');
    const agentResponse = await axios.get('/api/v1/agents', {params: {q: JSON.stringify({registration})}});
    if (agentResponse.status === 200 && agentResponse.data.data.length === 1) {
      const { id } = agentResponse.data.data[0];
      console.log(`removing agent with registration ${registration}`);
      await axios.delete(`/api/v1/agents/${id}`);
    }
  }
}


async function main()  {
  try {
    await removeAgent();
  } catch (err) {
    throw err;
  }
}

main()
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
