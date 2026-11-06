<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# <span style="color:#335bff">Android Labs</span>

This document provides some example guided usage scenarios for the **Android** Jenkins workloads. It should be used after the [workload_usage.md](workload_usage.md), [workload_usage_android.md](workload_usage_android.md) documents are read and understood.

## <span style="color:#335bff">Table of Contents<a name="table-of-contents"></a></span>

- [Prerequisites](#prerequisites)
- [Basic Labs](#basic-labs)
- [Advanced Lab 1: Build an Android Automotive App & Deploy to a Device](#app)
  - [1.1: Build the Road Reels Application](#apk-build)
  - [1.2: Launch Cuttlefish Virtual Device](#lab1-launch-cuttlefish)
  - [1.3: Install App on Cuttlefish Virtual Device using MTK Connect Tunnel connection from local machine](#apk-install)
  - [1.4: View Cuttlefish Virtual Device in Android Studio (via MTK Connect Tunnel)](#access-android-studio)
- [Advanced Lab 2: Additional Modifications to Android Code & Verification on Devices](#android-modifications)
  - [2.1 Edit Boot Animation](#boot-animation)
  - [2.2 Change Surface Colour](#surface-colour)
- [Advanced Lab 3: Override Make Commands to build HAL updates](#override-make)
- [Additional Suggestions](#additional-advanced)

> [!IMPORTANT]
> - The URLs referenced in the instructions reference an example domain `example.horizon-sdv.com`; replace these URLs with your domain
> - Jenkins Dashboard: https://example.horizon-sdv.com/jenkins/
> - Ensure that all Builds are executed using the appropriate [lunch target](../android/builds/aaos_builder.md#targets) for the desired device / android version / architecture.
> - see [Test Job Tips](workload_usage_android.md#test-job-tip)

## <span style="color:#335bff">Prerequisites<a name="prerequisites"></a></span>

1. **Platform deployed** — Horizon SDV cluster and services (including Jenkins) are available for your environment; see your deployment guide.
2. **Workload Setup** - completed as per [workload_setup.md](workload_setup.md).
3. **Developer tools** — As required (e.g. `git`, Google Cloud CLI, `adb` / `fastboot`).
4. **Workload Usage Documentation** - read and understood: [workload_usage.md](workload_usage.md), [workload_usage_android.md](workload_usage_android.md)

## <span style="color:#335bff">Basic Labs<a name="basic-labs"></a></span>

| Lab | Steps |
| -------- | -------- |
| 1. Android Studio Virtual Device | 1. [Build](workload_usage_android.md#build-devices) the SDK virtual device target <br>2. [Launch](workload_usage_android.md#devices-android-studio) the virtual device in Android Studio |
| 2. Cuttlefish Virtual Device  | 1. [Build](workload_usage_android.md#build-devices) the cuttlefish target <br>2. [Launch](workload_usage_android.md#devices-cuttlefish) the cuttlefish virtual device <br>3. [Access](workload_usage.md#appendix-mtk-connect-testbench-access-browser) the virtual device in MTK Connect  |
| 3. Test Cuttlefish Virtual Device | 1. [Build](workload_usage_android.md#build-devices) the cuttlefish target <br>2. [Launch & Run](workload_usage_android.md#testing-cts-default) the default CTS test suite on the cuttlefish virtual device |
| 4. Explicit Build with Gerrit Change | 1. Make a [change in Gerrit](workload_usage_android.md#gerrit-change-review) <br>- _use the provided example if required_<br>2. Perform an [Explicit Build](workload_usage_android.md#gerrit-build-explicit) with that Gerrit change <br> 3. Use training path 2 or 3 (excluding the Build step) to spin up a device & test the change |
| 5. Triggered Build (& Test) with Gerrit Change | 1. Make a [change in Gerrit](workload_usage_android.md#gerrit-change-review) <br>- _use the provided example if required_<br> 2. Perform a [Triggered Build](workload_usage_android.md#gerrit-build-triggered) with that Gerrit Change <br>3. Use the cuttlefish artifacts with training path 2 or 3 (excluding the Build step) to spin up a device & test the change |
| 6. AI Review of Failing Build | 1. [Perform an AI Review](workload_usage.md#ai-review-perform) on a build job <br>- _either use a known-to-fail target/revision or deliberately break the code and build that_ <br>2. Implement AI-suggested fix(es) as a [change in Gerrit](workload_usage_android.md#gerrit-change-review) <br>3. Perform an [Explicit Build](workload_usage_android.md#gerrit-build-explicit) with that Gerrit change <br> 4. Use training path 2 or 3 (excluding the Build step) to spin up a device & test the change| 


## <span style="color:#335bff">Advanced Lab 1: Build an Android Automotive App & Deploy to a Device<a name="app"></a></span>

**Learning Objective**

This lab builds upon the Google CodeLab ["Build and test a parked app for Android Automotive OS"](https://developer.android.com/codelabs/build-a-parked-app?hl=en#0) (Road Reels media application lab).
The lab won't be followed fully - there will be some modifications to / omissions of steps.

Instead of Section 4 (_Run the app in the Android Automotive OS emulator_) the following steps should be executed:


<details id="apk-build"><summary><b>Build the Road Reels Application</b></summary>

- Open [Build and test a parked app for Android Automotive OS](https://developer.android.com/codelabs/build-a-parked-app?hl=en#1) in your browser 
- Follow the instructions up to Section 3, i.e.
    - ```git clone https://github.com/android/car-codelabs.git```
        - Note: this should be done in the same environment as your Android Studio installation (e.g. if you are running Android Studio from Windows, don't clone into your wsl workspace).
    - Open Android Studio and import project by selecting the `car-codelabs/build-a-parked-app/start` directory
        - Note: Although, there is also an option to use the `car-codelabs/build-a-parked-app/end` directory, which includes the solution code, this is not helpful in this exercise because we want to show the application running on our own Horizon devices.
- Skip _Section 4: Run the app in the Android Automotive OS emulator_ (i.e. installing the Play Store images, using them to spin up an Android Automotive OS Android Virtual Device and running the Road Reels app on it).
- Build the Road Reels APK in Android Studio
    - Select → `Build` → `Generate App Bundles or APKs`→ `Generate APKs`
- Open the terminal session in Android Studio and locate the APK that was built. Save this location for use later.
    - e.g. in `app/build/outputs/apk/debug`

</details>

<details id="lab1-launch-cuttlefish"><summary><b>Launch Cuttlefish Virtual Device</b></summary>

This next part of the lab builds upon that application but allows user to install the application, using `adb`, through a MTK Connect tunnel connected to the Cuttlefish Virtual Device running in Horizon SDV.

- [Build a Cuttlefish Target](workload_usage_android.md#build-devices) if a build with the required artifacts is not already present
    - e.g. `aosp_cf_x86_64_auto-bp3a-userdebug` for `x86` using `android-16.0.0_r3`
- [Launch the Virtual Device](workload_usage_android.md#devices-cuttlefish) (using the build artifacts already created)
    - Ensure `CUTTLEFISH_INSTALL_WIFI` is enabled so that you will be able to play media files.
    - Set `NUM_INSTANCES` to 1
    - Wait for the `CVD Launcher` job to transition to `Keep Devices Alive` stage
    - This job will create an [MTK Connect Testbench](workload_usage.md#appendix-mtk-connect-testbench-access) which can be used to access the virutal device; take note of the testbench name.

</details>

<details id="apk-install"><summary><b>Install App on Cuttlefish Virtual Device using MTK Connect Tunnel connection from local machine</b></summary>

- Create a tunnel to the MTK Connect Testbench & Connect to Cuttlefish Device
    - Follow instructions to [set up a tunnel to the MTK Connect testbench](workload_usage.md#appendix-mtk-connect-testbench-access-tunnel).
    - When instructed to 'Open a command prompt', use the _Terminal Session_ in Android Studio
- Use the location of the Road Reels APK (saved from earlier) with the following command to install the app on the cuttlefish device
    - `adb install app-debug.apk`

</details>

<details id="access-android-studio"><summary><b>View Cuttlefish Virtual Device in Android Studio (via MTK Connect Tunnel)</b></summary>

With a tunnel connection to the MTK Connect Testbench already established and connection to the device initiated, the device can also be accessed interactively in Android Studio.
- Use `Device Manager` in Android Studio to establish a connection to the virtual device
- The UI & controls for the device should now be visible:
- The Road Reels application which was installed in the previous section can now be launched. 

</details>


## <span style="color:#335bff">Advanced Lab 2: Additional Modifications to Android Code & Verification on Devices<a name="android-modifications"></a></span>

Gerrit repos can be used to modify the standard Android code and subsequently build and test those changes on virtual or real devices.

See [here](workload_usage_android.md#gerrit) for more info on how to use Gerrit.

### <span style="color:#335bff">2.1 Edit Boot Animation<a name="boot-animation"></a></span>

This change updates the Android boot animation.

**AIM:** Use modified boot animation images in the boot process.

<details><summary><b>Clone Gerrit Repo</b></summary>

- In Gerrit select `BROWSE` → `Repositories` → `android/platform/packages/services/Car`

- Copy the `Clone with commit-msg hook`

- Paste the copied command to clone the repo (Note: use the copied command, not the example given here):
    <pre>git clone "https://example.horizon-sdv.com/gerrit/android/platform/packages/services/Car" && (cd "Launcher" && mkdir -p git rev-parse --git-dir/hooks/ && curl -Lo git rev-parse --git-dir/hooks/commit-msg https://example.horizon-sdv.com/gerrit/tools/hooks/commit-msg && chmod +x git rev-parse --git-dir/hooks/commit-msg)</pre>

</details>

<details><summary><b>Background Info</b></summary>

>[!NOTE]
> Refer to Google [README](https://android.googlesource.com/platform/packages/services/Car/+/refs/tags/android-14.0.0_r30/car_product/car_ui_portrait/bootanimation/README) and [FORMAT.md](https://android.googlesource.com/platform/frameworks/base/+/master/cmds/bootanimation/FORMAT.md) for further details on how the boot animation is implemented.

- Android Makefile to reference the boot animation: `Car/car_product/build/car_generic_system.mk`

- Android Boot Animation archive: `Car/car_product/bootanimations/bootanimation-832.zip`
    - this archive contains partX directories which contain the image files, and a description file, `desc.txt`.
        - `desc.txt` - describes the resolution of the boot animation and the PNG files, sequence (loops / delays). e.g.

            ```
            832 520 30. Resolution: <WIDTH> <HEIGHT> <FRAMERATE>
            c 1 30 part0 <TYPE> <COUNT> <PAUSE> <PATH>
            c 1 0 part1
            ...
            ```

        - The PNG files must be unique and sequence from `000.png` to `999.png`.

</details>

<details><summary><b>Create/Modify the boot animation sequence files</b></summary>

- Navigate to the cloned folder 
    - e.g. `cd Car`
    
- Identify the current branch & switch if required
    - `git branch --show-current`
    - e.g. `git checkout horizon/android-16.0.0_r3`

- Create or modify the image files:
  - _CREATE_: Create the PNG files and decide on a sequence. If using a video, then use a video splitter tool to convert video to frames.
    - Remember: PNG files are named `000.png` and increment, up to `999.png`
  - _MODIFY_: You may reuse the Android boot animation and simply decide to modify a few images.
- Create a new zip archive (with the `desc.txt` and `partX` directories and content) and store it in `Car/car_product/bootanimations/`, e.g.
  - `zip -0qry -i \*.txt \*.png \*.wav @ ../horizon-animation.zip *.txt part*`
- Update the `Car/car_product/build/car_generic_system.mk` makefile (or `Car/car_product/build/car.mk` for Pixel targets) to reference your new animation, e.g.
  ```
  # Boot animation
  PRODUCT_COPY_FILES += \
      packages/services/Car/car_product/bootanimations/horizon-animation.zip:system/media/bootanimation.zip
  ```

</details>

<details><summary><b>Push, Build, Test</b></summary>

>[!NOTE] This change updates the Android boot animation. Since cuttlefish devices accessed via MTK Connect will only be accessible after the device has booted, it is advisable to use either Android Studio targets (`Android SDK AVD`) or Pixel targets ( `tagoropro_car`) where the boot screen can be viewed.

- Save, commit and push the change for review: [instructions here](workload_usage_android.md#gerrit-change-review)
- Build with the change - either using an [Explicit Build](workload_usage_android.md#gerrit-build-explicit)(faster) or a [Triggered Build](workload_usage_android.md#gerrit-build-triggered).
- Test the change by verifying that the boot animation has changed on a booting device: [instructions here](workload_usage_android.md#gerrit-test)

</details>



### <span style="color:#335bff">2.2 Change Surface Colour<a name="surface-colour"></a></span>

This change updates the main Launcher window to add a colour overlay which will be clearly visible in the UI when the resulting device boots.

**AIM:** Use modified boot animation images in the boot process.

<details><summary><b>Clone Gerrit Repo</b></summary>

- In Gerrit select `BROWSE` → `Repositories` → `android/platform/frameworks/native`

- Copy the `Clone with commit-msg hook` and paste to clone the repo

</details>

<details><summary><b>Modify the surfaceflinger service</b></summary>

- Navigate to the cloned folder 
    - e.g. `cd native`
    
- Identify the current branch & switch if required
    - `git branch --show-current`
    - e.g. `git checkout horizon/android-16.0.0_r3`

- Edit `services/surfaceflinger/SurfaceFlinger.cpp` as follows:

  - <b>EDIT 1:</b> Search for the following line in `SurfaceFlinger::composite`:
    ```
    refreshArgs.devOptForceClientComposition = mDebugDisableHWC;
    ```
    and add the following code snippet before that line:
    ```
    refreshArgs.colorTransformMatrix =
            mat4(vec4{1.0f, 0.0f, 0.0f, 0.0f}, vec4{0.0f, -1.0f, 0.0f, 0.0f},
                 vec4{0.0f, 0.0f, -1.0f, 0.0f}, vec4{0.0f, 1.0f, 1.0f, 1.0f});
    ```
  - <b>EDIT 2:</b> Search for the following in `SurfaceFlinger::renderScreenImpl`:
    ```
    .updatingGeometryThisFrame = true,
    .colorTransformMatrix = calculateColorMatrix(colorSaturation),
    ```
    and replace it with the following:
    ```
    .updatingGeometryThisFrame = true,
    .colorTransformMatrix =
                mat4(vec4{1.0f, 0.0f, 0.0f, 0.0f}, vec4{0.0f, -1.0f, 0.0f, 0.0f},
                     vec4{0.0f, 0.0f, -1.0f, 0.0f}, vec4{0.0f, 1.0f, 1.0f, 1.0f}),
    ```
  - <b>EDIT 3:</b> There may be an unused variable that will show an error; do the following to avoid the error:
    Search for the following line:
    ```
    return output->getRenderSurface()->getClientTargetAcquireFence()`;
    ```
    and add the following code snippet before that line:
    ```
    base::StringPrintf("%.2fadb", colorSaturation);
    ```
    
</details>

<details><summary><b>Push, Build, Test</b></summary>

- Save, commit and push the change for review: [instructions here](workload_usage_android.md#gerrit-change-review)
- Build with the change - either using an [Explicit Build](workload_usage_android.md#gerrit-build-explicit)(faster) or a [Triggered Build](workload_usage_android.md#gerrit-build-triggered).
- Test the change by verifying that the launcher window colour has changed on a device: [instructions here](workload_usage_android.md#gerrit-test)

</details>


## <span style="color:#335bff">Advanced Lab 3: Override Make Commands to build HAL updates<a name="override-make"></a></span>

The Android Build jobs allow users to override the default make commands using the parameter [`OVERRIDE_MAKE_COMMAND`](../android/builds/aaos_builder.md#override_make_command) for scenarios where the default is not sufficient.
Example: when updates to the HAL have been made: `m android.hardware.automotive.vehicle.property-update-api && m dist`

**AIM:** To demonstrate how the user may override make commands, such as required when building HAL updates in `android/platform/hardware/interfaces`.

<details><summary><b>Clone Gerrit Repo</b></summary>

- In Gerrit select `BROWSE` → `Repositories` → `android/platform/hardware/interfaces`

- Copy the `Clone with commit-msg hook`

- Paste the copied command to clone the repo (Note: use the copied command, not the example given here):
    <pre>git clone "https://example.horizon-sdv.com/gerrit/android/platform/hardware/interfaces" && (cd "Launcher" && mkdir -p git rev-parse --git-dir/hooks/ && curl -Lo git rev-parse --git-dir/hooks/commit-msg https://example.horizon-sdv.com/gerrit/tools/hooks/commit-msg && chmod +x git rev-parse --git-dir/hooks/commit-msg)</pre>

</details>

<details><summary><b>Modify the Code to include a New Property</b></summary>

>[!NOTE:] This example is provided in order to demonstrate the requirment for a build override mechanism; users can make their own code changes.

- Navigate to the cloned folder 
    - e.g. `cd interfaces`
- Identify the current branch
    - `git branch --show-current`
- Switch to `horizon/android-16.0.0_r3` branch, if not already on it
    - e.g. `git checkout horizon/android-16.0.0_r3`

- Modify the files as per following `git diff` (remove the new lines `+` markers):
  ```
  diff --git a/automotive/vehicle/aidl/impl/current/default_config/config/DefaultProperties.json b/automotive/vehicle/aidl/impl/current/default_config/config/DefaultProperties.json
  index 665c10e8e3..0a4ae2e0c6 100644
  --- a/automotive/vehicle/aidl/impl/current/default_config/config/DefaultProperties.json
  +++ b/automotive/vehicle/aidl/impl/current/default_config/config/DefaultProperties.json
  @@ -1,6 +1,12 @@
   {
       "apiVersion": 1,
       "properties": [
  +        {
  +            "property": "VehicleProperty::INFO_HORIZON_SDV",
  +            "defaultValue": {
  +                "stringValue": "HORIZON-SDV-LAB"
  +            }
  +        },
           {
               "property": "VehicleProperty::INFO_FUEL_CAPACITY",
               "defaultValue": {
  diff --git a/automotive/vehicle/aidl_property/android/hardware/automotive/vehicle/VehicleProperty.aidl b/automotive/vehicle/aidl_property/android/hardware/automotive/vehicle/VehicleProperty.aidl
  index acb6aeb5a1..e2c5c99ecb 100644
  --- a/automotive/vehicle/aidl_property/android/hardware/automotive/vehicle/VehicleProperty.aidl
  +++ b/automotive/vehicle/aidl_property/android/hardware/automotive/vehicle/VehicleProperty.aidl
  @@ -47,6 +47,15 @@ enum VehicleProperty {
        * This property must never be used/supported.
        */
       INVALID = 0x00000000,
  +
  +    /**
  +     * Horizon SDV lab property
  +     *
  +     * @change_mode VehiclePropertyChangeMode.STATIC
  +     * @access VehiclePropertyAccess.READ
  +     */
  +    INFO_HORIZON_SDV = 0x1000 + 0x10000000 + 0x01000000
  +            + 0x00100000, // VehiclePropertyGroup:SYSTEM,VehicleArea:GLOBAL,VehiclePropertyType:STRING
       /**
        * VIN of vehicle
        *
  ```

- Save the file(s)

</details>


<details><summary><b>Push Changes for Review</b></summary>

Commit and push the change for review: [instructions here](workload_usage_android.md#gerrit-change-review)

</details>

<details><summary><b>Build the Change</b></summary>

- Build the change using an [Explicit Build](workload_usage_android.md#gerrit-build-explicit) (a Triggered Build using the _Gerrit_ job will fail because the AIDL must be rebuilt). The build job should use the following parameters (as well as specifying the appropriate Gerrit change as per instructions in the link provided):
    - `AAOS_LUNCH_TARGET`: `aosp_cf_x86_64_auto-bp3a-userdebug`
    - `OVERRIDE_MAKE_COMMAND`: `m android.hardware.automotive.vehicle.property-update-api && m dist`

- The build console log will show AIDL update.

</details>

<details><summary><b>Test the Change</b></summary>

- [Build a Cuttlefish Target](workload_usage_android.md#build-devices) if a build with the required artifacts is not already present
    - e.g. `aosp_cf_x86_64_auto-bp3a-userdebug` for `x86` using `android-16.0.0_r3`
- [Launch the Virtual Device](workload_usage_android.md#devices-cuttlefish) (using the build artifacts already created)
    - Ensure `CUTTLEFISH_INSTALL_WIFI` is enabled so that you will be able to play media files.
    - Set `NUM_INSTANCES` to 1
    - Wait for the `CVD Launcher` job to transition to `Keep Devices Alive` stage
    - This job will create an [MTK Connect Testbench](workload_usage.md#appendix-mtk-connect-testbench-access) which can be used to access the virutal device; take note of the testbench name.


- Launch a [cuttlefish device](workload_usage_android.md#devices-cuttlefish) using the build artifacts
- View the device in [MTK Connect](workload_usage.md#appendix-mtk-connect-testbench-access-browser) via a browser 
- Open an adb terminal to the device and execute the following command:
    `dumpsys car_service get-property-value 0x11101000`
    - The example change shows the property as not supported in HAL (`INFO_HORIZON_SDV(0x11101000) not supported by HAL`) but it recognises the property. That is expected because it is not the purpose of this lab to provide a working HAL update, rather demonstrate the build command override.

</details>


## <span style="color:#335bff">Additional Suggestions<a name="additional-advanced"></a></span>

There are many example applications available online that can be experimented with, such as:
- Google Code Labs:
  - [Car App Library Fundamentals](https://developer.android.com/codelabs/car-app-library-fundamentals?hl=en#0)
  - [Accessibility Testing with Espresso](https://developer.android.com/codelabs/a11y-testing-espresso#0)
 - [Android Car Samples on GitHub](https://github.com/android/car-samples)

Another potential area to explore includes:

- Working with newer Android revisions, such as:
  - Using Horizon SDV Gerrit-supported versions
  - Building against the upstream Google AOSP Gerrit and its revisions

> [!IMPORTANT]
> Please note that while there are no version restrictions (other than those imposed by the Gerrit job), lunch targets are still subject to certain limitations. Specifically, they are restricted to the targets defined in the [Build Targets](workload_usage_android.md#build-targets) section of the workload_usage_android.md document.
