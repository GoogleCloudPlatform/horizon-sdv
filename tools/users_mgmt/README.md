# Access Control Management Horizon

This is a tool designed to handle access management on GCP level.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [Horizon Roles](#horizon-roles)
- [Input data](#input-data)
- [Output data](#output-files)
- [Documentation](#documentation-generation)

## Overview

The *Access Control Management Horizon* project provides a Python script and classes for managing access management on GCP level. It handles Horizon roles and their mapping to GCP roles. This is useful in cases where roles within Horizon need to be mapped to specific permissions or roles in GCP, allowing for more efficient access control management in a cloud environment. Additionally it can directly manage user's GCP roles (e.g. delete, preview, assign).

## Installation

To install the required dependencies for the project:

1. Clone the repository:

    ```bash
    git clone https://github.com/AGBG-ASG/acn-horizon-sdv.git
    ```

2. Navigate to the script directory:

    ```bash
    cd tools/users_mgmt/
    ```

3. Configure environment.
    
    Run the setup script to create a virtual environment (in `venv/` directory) and install all required dependencies from `requirements.txt`.

    - **Windows**

        Execute the script using **WSL (Windows Subsystem for Linux)** or a Bash-compatible terminal like **Git Bash**. 

    - **Linux/macOS/WSL**
    
        ```bash
        bash setup.sh
        source venv/bin/activate
        ```

4. Script execution:

    ```bash
    python src/access_control.py
    ```

5. Post script execution action.

    When finished, deactivate the virtual environment by running:

    ```bash
    deactivate
    ```

## Usage

To use the scripts, simply execute the script access_control.py with appropriate parameters. 
eg: `python src/access_control.py` 

### GCP authentication

Since the script modifies GCP project configurations, authentication with the Google Cloud Platform is required. The script handles authentication automatically, but if any issues arise—particularly those related to previously saved credentials—it’s recommended to re-authenticate. You can do this by running the command `gcloud auth application-default revoke` in your terminal, then re-running the script. Additionally, to access the script’s full functionality, the user executing it must have the `resourcemanager.projects.setIamPolicy` permission.


### Available parameters:

- `--help (-h)`
    Displays help: script overview, possible parameters.

- `--project (-p)`
    Project id. If parameter is not provided, script uses default value `sdva-2108202401`.

- `--output (-out)`
    Directory to store output files. Provided directory can be relative or non-relative path. If parameter is not provided, script uses default value `../output/`.

- `--creds (-cr)`
    Name of file where the credentials are stored. If parameter is not provided, script uses default value `application_default_credentials.json`.

- `--operations (-op)`  
    Path to a JSON file containing a list of operations to be performed.  
    The operations could include actions like retrieving users, assigning roles, and more.

- `--horizon_roles (-hr)`  
    Path to a JSON file containing a list of Horizon roles and their respective GCP role mappings. This file is required for role management operations, as it defines the mapping between Horizon roles and GCP roles.

- `--check (-c)`  
    Path to a JSON file that contains users and their mapped Horizon roles. This parameter performs a check and compares the configuration in the provided JSON file with the actual configuration in GCP. The file must also include the `--horizon_roles` parameter to ensure proper comparison.

- `--users (-u)`  
    Path to a JSON file containing a list of users and the Horizon roles assigned to them. This file should also include the `--horizon_roles` parameter for proper mapping of GCP roles.  

    The file specifies the users (or groups of users) and the Horizon roles that they should have. If a group of users is defined, the roles are assigned to each user in the group.

- `--force (-f)`  
    Flag used for users' role mapping (it can be used with `--users (-u)` parameter). If set, the user's existing roles will be overwritten. If not set, the roles will be added to the user without affecting their existing roles.

- `--backup (-bu)`  
    Flag to create a backup json file which contains list of users and gcp roles assigned to them.

- `--log (-l)`
    Parameter to set level of logging information. Possible values: *DEBUG*, *INFO*, *WARNING*, *ERROR*. Default value is *INFO*.

### Script Execution Logic

#### Horizon Roles Mapping
If the `--horizon_roles` parameter is provided, the script loads the mapping between Horizon roles and GCP roles. This mapping is required for role management actions.

Execution:
```
python src/access_control.py -hr horizon_roles.json
```
Executing script with only `-hr` parameter does not perform actual outcome action. It is required with other parameters. 

#### User Role Assignment
This parameter requires providing parameter `--horizon_roles` and can be used with `--force` flag. <br>The script checks if the `user` or `group_role` fields are present in the provided users' JSON file.
   - If the `user` field is present, the script will assign the corresponding Horizon roles to the user based on the `--horizon_roles` mapping.
   - If the `group_role` field is present, the script will assign the Horizon role to the specified users in the group.

Execution:

To **add** roles to users execute:
```
python src/access_control.py -p project_id -u users.json -hr horizon_roles.json 
```
To **overwrite** roles to users execute:
```
python src/access_control.py -p project_id -u users.json -hr horizon_roles.json -f
```
Project ID parameter is a good practise to ensure the correct project will be updated.  

#### Check provided configuration
This parameter requires providing parameter `--horizon_roles`. <br>If the `--check` parameter is provided, the script compares the configuration in the supplied JSON file (containing users and their assigned Horizon roles) with the current state in the Google Cloud Project.

Execution:

```
python src/access_control.py -p project_id -c users.json -hr horizon_roles.json 
```

#### Operations Execution
If the `--operations` file is provided, the script processes each operation in the file sequentially.

Execution:

```
python src/access_control.py -p qwiklabs-asl-02-1779837da9b6 --op operations.json
```


### Example Execution

```bash
python access_control_horizon.py --horizon_roles /path/to/horizon_roles.json --users /path/to/users.json --force --log DEBUG
```

In this example:
- `--horizon_roles`: Specifies the path to the JSON file containing the Horizon to GCP role mappings.
- `--users`: Specifies the path to the JSON file containing users and their assigned Horizon roles.
- `--force`: Forces the script to overwrite existing roles for users.
- `--log`: Log level is set to *DEBUG*, all possible information will be saved to log.

The script will reassign the roles to the users as defined in the users' JSON file.

## Horizon Roles

Horizon roles are configured in file `horizon_roles.json`. Path to file is provided in script argument. Roles can be modified by edditing the file.

Configured Horizon roles:

1. horizon_admin
    - "roles/iam.securityAdmin" - required for updating other users' GCP roles (has ability to get and set any IAM policy).
    - "roles/editor" - View, create, update, and delete most Google Cloud resources.
    - "roles/iap.tunnelResourceAccessor" - Access Tunnel resources which use Identity-Aware Proxy.
    - "roles/compute.osAdminLogin" -  Access to log in to a Compute Engine instance as an administrator user.
    - "roles/container.admin" - Full management of Kubernetes Clusters and their Kubernetes API objects.
    - "roles/storage.objectUser" - Access to create, read, update and delete objects and multipart uploads in GCS.

2. horizon_developer
    - "roles/iap.tunnelResourceAccessor" - Access Tunnel resources which use Identity-Aware Proxy.
    - "roles/compute.osAdminLogin" - Access to log in to a Compute Engine instance as an administrator user.
    - "roles/container.admin" - Full management of Kubernetes Clusters and their Kubernetes API objects.

3. horizon_viewer
    - "roles/viewer" - View most Google Cloud resources.

4. horizon_workload_user
    - "roles/storage.objectUser" - Access to create, read, update and delete objects and multipart uploads in GCS.

## Input data

Depending on the required functionality, script can take as input JSON files with neccessary configurations and information.

### Horizon Roles JSON File

The JSON file required with parameter `--horizon_roles` must contain mappings between Horizon roles and their corresponding GCP roles. The JSON file should have following structure:
```json
[
    {
        "name": "horizon_role1",
        "gcp_roles": ["roles/gcp_role1", "roles/gcp_role2"]
    },
    {
        "name": "horizon_role2",
        "gcp_roles": ["roles/gcp_role3"]
    }
]
```

### User JSON File

The JSON file required with parameter `--users` must contain list of users and the Horizon roles assigned to them. The JSON file should have following structure:
```json
[
    {
        "user": "user1@accenture.com",
        "horizon_roles": ["horizon_developer"]
    },
    {
        "group_role": "horizon_viewer",
        "users":["user2@accenture.com", "user3@accenture.com"]
    },
    {
        "user": "user4@accenture.com",
        "horizon_roles": ["horizon_admin"]
    }
]
```

The same file is provided with parameter `--check`

### Operations JSON File

The `--operations` parameter allows you to pass a JSON file containing a list of operations to perform. The operations may include:

- `GET_USER`: Retrieves information about a specific user (requires `user` field).
- `GET_ALL_ROLES`: Retrieves a list of all roles.
- `GET_ALL_ROLES_WITH_USERS`: Retrieves a list of roles along with the users assigned to them.
- `GET_ROLES_WITH_USERS`: Retrieves a list of specific roles and the users assigned to them (requires `roles` field).
- `GET_ROLE_INFO`: Retrieves information about a specific role (requires  `role` field).
- `SET_ROLE_TO_USER`: Assigns a role to a user (requires `user` and `role` fields).
- `DELETE_ROLE_FROM_USER`: Removes a role from a user (requires `user` and `role` fields).
- `DELETE_ALL_ROLES_FROM_USER`: Removes all roles from a user (requires `user` field).

Example operations file:
```json
[
    {"operation": "GET_ALL_USERS"},
    {"operation": "GET_USER", "user": "user@accenture.com"},
    {"operation": "GET_ROLE_INFO", "role": "storage.objectViewer"},
    {"operation": "SET_ROLE_TO_USER", "user": "user@accenture.com", "role": "viewer"}
]
```

## Output data

Generated files are stored in the directory `\users_mgmt\output\`. The directory is pointed in global variable `OUTPUT_DIR` in `access_control.py` script.

The script generates output files as a result of executing operations. Each operation, except for SET_ROLE_TO_USER and DELETE_ROLE_FROM_USER, produces a JSON file with the corresponding output data.

Additionally, the script logs detailed information about the execution process, including any errors or successful steps taken. The logs are saved in a `LOG.log` file.

## Documentation

This project uses [Sphinx](https://www.sphinx-doc.org/) to generate project documentation. The documentation is written in reStructuredText format and can be built into HTML files. Below are the steps for generating the documentation locally.

### Documentation generation

To generate the HTML documentation, follow these steps:

1. Navigate to the `docs/` directory:

    ```bash
    cd docs
    ```

3. Build the HTML documentation:

    ```bash
    make html
    ```

   The generated documentation will be available in the `docs/_build/html/` directory.

### Documentation Structure

The documentation includes the following key sections:

- **Index (`index.rst`)**: The main entry point for the documentation, which includes the table of contents and links to various sections.
- **Modules (`modules.rst`)**: Provides an overview of the modules in the `access_control_horizon` codebase, documenting the functionality of each module and class.
