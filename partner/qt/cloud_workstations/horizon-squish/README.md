# Squish for Horizon

This cloudworkstation is meant to provide the user with a Squish installation ready to be used.

For more information regarding Squish and how to use the specifics please have a look at the [official documentation](https://doc.qt.io/squish/squish-for-android-tutorials.html).

## Guide: Additions to the `values-jenkins.yaml`

Please provide the following additions to the [`values-jenkins.yaml`](../../../../gitops/workloads/values-jenkins.yaml):

- under `controller:JCasC:configScripts:welcome-message:globalNodeProperties:envVars:env`:
```yaml
- key: "CLOUD_WS_HORIZON_SQUISH_FOR_ANDROID_IMAGE_NAME"
  value: {{ .Values.config.workloads.cloudWorkstations.workstationPresets.wsImages.horizonSquishForAndroid.image }}
```

- under `config:workloads:cloudWorkstations:workstationPresets:wsImages`:
```yaml
horizonSquishForAndroid:
    image: "horizon-sdv/cloud-ws-images/horizon-squish-for-android"
```

## Guide: Providing the Squish License Key

This document outlines the steps required to provide the Squish License Key to the automated build pipeline. The pipeline uses Jenkins for orchestration and Kaniko for secure, rootless Docker image builds.

### Prerequisites

- Access to the Jenkins instance hosting the horizon-squish-for-android pipeline.

- A valid Squish License Key (provided by [The Qt Company](https://www.qt.io/quality-assurance/download)).

- Permissions to manage Jenkins Credentials within the relevant folder or global scope.

### Add the Secret to Jenkins

The pipeline is configured to look for a specific credential ID. You must store your license key in Jenkins to make it available to the build environment.

1. Navigate to your Jenkins dashboard.

2. Go to Manage Jenkins > Credentials.

3. Select the appropriate Domain (usually "(global)").

4. Click Add Credentials.

5. Configure the following settings:

    - Kind: Secret text

    - Scope: Global (or limited to the specific folder if applicable)

    - Secret: Paste your Squish License Key here.

    - ID: squish-license-file  (This must match exactly)

    - Description: License key for Squish for Android workstation builds.

6. Click Create.

> **Security Note**: The license key is only available during the RUN command in the Dockerfile. It is not stored in the final image layers or environment variables, ensuring your license remains private.

### Troubleshooting

If the build fails during the "Create Docker Image" stage, check the following:

- Credential ID: Ensure the ID in Jenkins is exactly squish-license-file.

- Expired License: Verify the license string itself is still valid with The Qt Company.

- Kaniko Logs: Check the Jenkins console output. If you see an error like secret id=squish-license not found, verify that the --secret flag is correctly set in the sh block of the Jenkinsfile.