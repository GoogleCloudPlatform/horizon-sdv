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


"""
access_control.py

Access control script for a Platform Administrator.
Script takes input parameter:
--operations (-op) - parameter requires path to JSON file which contains list of operations that shall be performed.
--horizon_roles (-hr) -  parameter requires path to JSON file which contains list of Horizon roles with GCP roles mapping.
--users  (-u) - parameter requires path to JSON file which contains list of users and Horizon roles assigned to them that they shall have.
                It is also required to provide parameter --horizon_roles.
--force  (-f) - used for users' roles mapping. If flag is set than user's roles will be overwritten.
                If flag is not set roles will be added to user with no interference to their existing roles.
--check  (-c) - parameter requires path to JSON file which contains list of users and mapped Horizon roles (as in parameter --users).
                Performs a check and compares configuration in provided JSON file with Horizon roles mapping and existing configuration in Google Cloud Platform.
                It is also required to provide parameter --horizon_roles.
--backup (-bu) - Flag to create a backup json file which contains list of users and gcp roles assigned to them.

Author: Accenture
Date: March 2025
"""
import os
import sys
import argparse
import subprocess
import logging
from enum import Enum, auto
import google.auth
from googleapiclient import discovery
import requests

from operations import Operation
from roles_mgmt import RolesMgmt


PROJECT_ID = "sdva-2108202401"
CREDENTIALS_FILENAME = "application_default_credentials.json"
OUTPUT_DIR = "output"

class LogLvl(Enum):
    """Log Level Enum
    Possible levels of logging.
    """
    DEBUG = logging.DEBUG
    INFO = logging.INFO
    WARNING = logging.WARNING
    ERROR = logging.ERROR


def logging_config():
    """Configure logger for the script.

    Returns:
        logging.Logger: logger
    """

    log_formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

    # File handler to write logs to a file
    log_file_handler = logging.FileHandler("LOG.log", mode="w")
    log_file_handler.setFormatter(log_formatter)

    # Console handler to logger.info logs to stdout
    log_console_handler = logging.StreamHandler()
    log_console_handler.setFormatter(log_formatter)

    logger = logging.getLogger('MyLogger')
    logger.setLevel(LogLvl.INFO.value)
    logger.addHandler(log_file_handler)
    logger.addHandler(log_console_handler)

    return logger

def log_lvl_update(lvl):
    """_summary_

    _extended_summary_
    Args:
        lvl (string): Level of output log. Shall be one of the values: DEBUG, INFO, WARNING, ERROR.
    """    
    try:
        logger.setLevel(lvl)
    except ValueError as e:
        logger.error(f"Incorect log level was provided: {lvl}. Log level will remain {LogLvl(logger.level).name} \nError: {e}")
        

def check_credentials():
    """Checks if credentials were already created.

    It simplifies authentication by checking common locations for credentials, such as:
    - The GOOGLE_APPLICATION_CREDENTIALS environment variable (for service account keys).
    - Credentials obtained from gcloud auth application-default login.

    Returns:
        bool: result of check if credentials exist
    """

    # Check credentials from the GOOGLE_APPLICATION_CREDENTIALS environment variable.
    # GOOGLE_APPLICATION_CREDENTIALS stores path to credentials file.
    if os.environ.get("GOOGLE_APPLICATION_CREDENTIALS"):
        return True
    else:
        # Check credentials from the Cloud SDK.
        # Check if the path is explicitly set. If yes, check if credentials are stored in given path.
        env_var = os.environ.get("CLOUDSDK_CONFIG")
        if env_var:
            credentials_file_path = os.path.join(env_var, CREDENTIALS_FILENAME)
            if os.path.isfile(credentials_file_path):
                return True
        else:
            # Check manually (not using environment variable) if credentials files exists.
            if os.name != "nt":
                # Check credentials on Non-windows system. They should be stored at ~/.config/gcloud
                credentials_file_path = os.path.join(os.path.expanduser(
                    "~"), ".config", "gcloud", CREDENTIALS_FILENAME)
                if os.path.isfile(credentials_file_path):
                    return True
            else:
                # Check credentials on Windows systems. Config should stored at %APPDATA%/gcloud
                env_var = os.environ.get("APPDATA")
                if env_var:
                    credentials_file_path = os.path.join(
                        env_var, "gcloud", CREDENTIALS_FILENAME)
                    if os.path.isfile(credentials_file_path):
                        return True

    return False


def authentication():
    """Authenticate user.

    Make sure the gcloud CLI from https://cloud.google.com/sdk is installed.

    It uses Application Default Credentials.
    Automatically finds your credentials (like a service account or ADC credentials) based on the environment.
    If not already done, runs `gcloud auth application-default login` command which opens browser to authenticate.
    If that fails, runs `gcloud auth application-default login --no-browser` command which lets authenticate without access to a web browser.
    Generates a link which should be run on a machine with a web browser and copy the output back in the command line.


    Returns:
        tuple (bool, google.auth.credentials.Credentials): operation status (True if operation succeeded) and the current environment's credentials
    """

    return_status = False
    credentials = None
    user_id = None

    logger.info("------")
    if check_credentials():
        logger.info("Credentials already exist.")
        try:
            credentials, proj_id = google.auth.default()
            credentials.refresh(google.auth.transport.requests.Request())
            return_status = True
        except google.auth.exceptions.DefaultCredentialsError as e:
            logger.critical(f"Credentials missing or invalid. Running login flow. \nError during authentication: {e}")
        else:
            logger.info(f"You are authenticated.")

    if not credentials:
        logger.info(f"There are no credentials. You will need to log in.")
        try:
            subprocess.run(["gcloud", "auth", "application-default", "login"], check=True)
        except subprocess.CalledProcessError as e:
            logger.info("Another try to authenticate.")
            try:
                result = subprocess.run(
                    ["gcloud", "auth", "application-default", "login", "--no-launch-browser"], check=True)
                result.check_returncode()
            except Exception as e:
                logger.critical(f"""There were three attempts to authenticate:
                      1. Automatically check saved credentials.
                      2. Login with browser.
                      3. Provide an url to login using browser on machine with connection to internet.
                      Authentication failed. Fix it.
                      Returned error:
                      {e}.""")
                raise
            else:
                credentials, proj_id = google.auth.default()
                return_status = True
                logger.info(f"You are authenticated.")
        except Exception as e:
            print("3")
            logger.critical(f"Error during authentication: {e}")
        else:
            if check_credentials():
                credentials, proj_id = google.auth.default()
                return_status = True
                logger.info(f"You are authenticated.")
            else:
                logger.critical("There was a problem with authentication. You are not authenticated.")

    if credentials:
        try:
            subprocess.run(["gcloud", "config", "set", "project", PROJECT_ID], check=True)
            subprocess.run(["gcloud", "auth", "application-default", "set-quota-project", PROJECT_ID], check=True)
        except Exception as e:
            logger.critical("There was a problem when setting project id and quota project id. \nReturned error: \n{e}.")
            
        logger.info(f"Project details: project id {proj_id}, quota project: {credentials.quota_project_id}")

        # RETRIEVE ID OF USER EXECUTING THE SCRIPT #
        credentials.refresh(google.auth.transport.requests.Request())
        id_info = requests.get(
            "https://www.googleapis.com/oauth2/v3/tokeninfo",
            params={"access_token": credentials.token}
        ).json()

        user_id = id_info.get("email")
        logger.info(f"User executing the script: {user_id}")

    return return_status, credentials, user_id


def script_arguments_project_setup():
    """Retrieve script arguments.

    retrieve arguments provided with the script and update project configuration.

    Returns:
        argparse.Namespace: Arguments provided with the script
    """

    global PROJECT_ID, OUTPUT_DIR, CREDENTIALS_FILENAME
    parser = argparse.ArgumentParser(
        description="Script for managing access management on GCP level. USe the script with chosen arguments.")
    parser.add_argument("-p", "--project",
                        help=f"Project id. Default value: {PROJECT_ID}")
    parser.add_argument("-out", "--output",
                        help=f"Directory to store output files. Default value: {OUTPUT_DIR}")
    parser.add_argument("-cr", "--creds",
                        help=f"Name of file where the credentials are stored. Default value: {CREDENTIALS_FILENAME}")
    parser.add_argument("-op", "--operations",
                        help="Path to json file which contains operations list to perform.")
    parser.add_argument("-hr", "--horizon_roles",
                        help="Path to json file which contains list of Horizon roles with GCP roles mapping.")
    parser.add_argument("-u", "--users",
                        help="Path to json file which contains list of users and assigned to them Horizon roles that they shall have.")
    parser.add_argument("-f", "--force", action="store_true",
                        help="Flag used for users' roles mapping. \nIf flag is set than user's roles will be overwritten. \nIf flag is not set roles will be added to user with no interference to their existing roles.")
    parser.add_argument("-c", "--check",
                        help="Path to json file which contains list of users and Horizon roles assigned to them. Perform a check and compare configuration in provided Json file with Horizon roles mapping and existing configuration in Google Cloud Platform.")
    parser.add_argument("-bu", "--backup", action="store_true",
                        help="Flag to create a backup json file which contains list of users and gcp roles assigned to them.")
    parser.add_argument("-l", "--log",
                        help=f"Level of logging information. Possible values: {LogLvl.DEBUG.name}, {LogLvl.INFO.name}, {LogLvl.WARNING.name}, {LogLvl.ERROR.name}")
    arguments = parser.parse_args()

    # Check if arguments were provided to the script
    if len(sys.argv) == 1:
        logger.warning("No arguments were provided to the script.")
        arguments = None
    else:
        if arguments.log:
            log_lvl_update(lvl=arguments.log)
        
        if arguments.project:
            PROJECT_ID = arguments.project

        if arguments.output:
            OUTPUT_DIR = arguments.output

        if arguments.creds:
            CREDENTIALS_FILENAME = arguments.creds

    return arguments


def script_arguments_handler(arguments, operations_management, roles_management):
    """Handler for script arguments

    Handle arguments provided to the script.

    Args:
        arguments (argparse.Namespace): Arguments provided with the script
        operations_management (operations.Operation): Operations class instance
        roles_management (roles_mgmt.RolesMgmt): Roles Management class instance
    """

    operation_result = False

    if arguments.horizon_roles:
        operation_result = roles_management.horizon_roles_mgmt(horizon_roles_file_path=arguments.horizon_roles)

    if arguments.backup:
        logger.info("Backup file will be created")
        operation_result = operations_management.operations_handler(operations_management.Operations.GET_USER.name, user="*")

    if arguments.check:
        roles_management.check_users_configurations(users_list_file_path=arguments.check)

    if arguments.users:
        if operation_result:
            operation_result = roles_management.users_mgmt(users_list_file_path=arguments.users, force_overwrite=arguments.force)
        else:
            logger.error(
                "To perform users mapping functionality provided in parameter `--users`, parameter `--horizon_roles` neads to be provided correctly.")

    if arguments.operations:
        operations_management.retrieve_operations_list_from_json(operations_file_path=arguments.operations)


if __name__ == '__main__':

    # Logging setup
    logger = logging_config()
    
    # RETRIEVE SCRIPT ARGUMENTS + PROJECT SETUP #
    script_arguments = script_arguments_project_setup()

    logger.info(f"ADC path: {os.environ.get('GOOGLE_APPLICATION_CREDENTIALS', 'default ADC file used')}")
    logger.info(f"Python executable: {sys.executable}")
    logger.info(f"gcloud config dir: {os.environ.get('CLOUDSDK_CONFIG', '~/.config/gcloud')}")
    logger.info("Start script execution")

    if script_arguments:
        # AUTHENTICATION #
        operation_status, credentials, user_id = authentication()

        # HANDLING SCRIPT ARGUMENTS #
        if operation_status:
            service = discovery.build(serviceName='iam', version='v1', credentials=credentials)

            operations_mgmt = Operation(service=service, project_id=PROJECT_ID, logger=logger, output_dir=OUTPUT_DIR, exec_user_id=user_id)
            roles_mgmt = RolesMgmt(operation_manager=operations_mgmt, logger=logger, exec_user_id=user_id)

            script_arguments_handler(arguments=script_arguments, operations_management=operations_mgmt, roles_management=roles_mgmt)

            logger.info("------")
    logger.info("End script execution")
