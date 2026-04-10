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

## Guide: Providing the Squish License Key and URL

This document outlines the steps required to provide the Squish License Key and the download URL to the automated build pipeline.

### Prerequisites

- A valid Squish License Key (provided by [The Qt Company](https://www.qt.io/quality-assurance/download)).

### Add the Secret and URL to the Dockerfile

1. Open the [Dockerfile](Dockerfile)

2. Navigate to the `RUN` command

3. Replace `<URL>` and `<LICENSE_KEY>` with the once provided by The Qt Company

### Troubleshooting

If the build fails during the "Create Docker Image" stage, check the following:

- Expired License: Verify the license string itself is still valid with The Qt Company.

- URL still valid: Verify the URL is still valid with The Qt Company.