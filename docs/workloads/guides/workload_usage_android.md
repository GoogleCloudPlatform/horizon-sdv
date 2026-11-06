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

# <span style="color:#335bff">Workload Usage - Android</span>

This document covers the usage of **Android** Jenkins workloads.

It includes pointers to workload documentation under [`docs/workloads/`](../../workloads/) (job parameters and scripts are described there and in `workloads/*/pipelines/**/README.md`).

> **Note:** This setup document combines with the [workload_setup.md](workload_setup.md), [workload_usage.md](workload_usage.md) and [android_labs.md](android_labs.md) documents to supersede the former Android `docs/workloads/android/guides/developer_guide.md` (deprecated). 

> [!IMPORTANT]
> - Android versions are updated regularly. Users should verify which releases are currently supported. Exercises in this guide may reference older versions; please update them to the latest supported release as appropriate.
> - When working with Cuttlefish, please be aware that the latest supported versions change frequently. The examples provided in this guide may become outdated, as <a href=https://github.com/google/android-cuttlefish/tags>tags</a> are updated regularly.

[ ============================================================================= ]::

## <span style="color:#335bff">Table of Contents</span>

- [Prerequisites](#prerequisites)
- [ANDROID BUILDS](#builds)
  - [Building for Various Devices](#build-devices)
    - [Supported Devices (& associated values)](#build-devices-supported)
    - [1. Build the Target](#build-device-target)
    - [2. Download / Note the Build Artifacts](#build-get-artifacts)
  - [Build the CTS Test Suite](#build-cts)
  - [Other Build Topics](#build-other)
    - [Build Targets](#build-targets)
    - [Over-riding Make Commands in Builds](#build-make)
    - [Build a Gerrit Change](#build-gerrit-change)
- [ANDROID DEVICES](#devices)
  - [Android Studio Virtual Device](#devices-android-studio)
    - [Install the Virtual Device](#devices-android-studio-install)
    - [Launch the Virtual Device](#devices-android-studio-launch)
    - [Uninstall the Virtual Device](#devices-android-studio-uninstall)
  - [Cuttlefish Virtual Device](#devices-cuttlefish)
    - [Launch the Virtual Device](#devices-cuttlefish-launch)
  - [Pixel Tablet Device](#devices-pixel)
    - [Unlock Device](#devices-pixel-unlock)
    - [Flash Device](#devices-pixel-flash)
    - [Tips](#pixel-tips)
  - [Raspberrry Pi Device](#devices-rpi)
    - [Create a Flashable Image](#devices-rpi-create-image)
    - [Flash the Device](#devices-rpi-flash)
- [ANDROID TESTING](#testing)
  - [Test job prerequisites](#test-job-tip)
  - [Automated Testing - CTS](#testing-cts)
    - [Run Default CTS on Cuttlefish Virtual Device(s)](#testing-cts-default)
    - [Run Custom CTS on Cuttlefish Virtual Device(s)](#testing-cts-custom)
    - [CTS Artifacts](#testing-cts-artifacts)
    - [CTS Tips](#testing-cts-tips)
  - [Manual Testing - MTK Connect with Cuttlefish Device](#testing-mtk-connect-cuttlefish)
  - [Manual Testing - Android Studio](#testing-android-studio)
- [ANDROID CHANGE USING GERRIT](#gerrit)
  - [Creating a Gerrit Change for Review](#gerrit-change-review)
    - [Clone Gerrit Repo](#gerrit-change-clone)
    - [Modify the Code](#gerrit-change-edit)
    - [Push Changes for Review](#gerrit-change-push)
  - [Gerrit Change Identification](#gerrit-change-id)
    - [Using Single Change Parameters](#gerrit-change-id-single)
    - [Using _Topic_ Property](#gerrit-change-id-topic)
  - [Build with Gerrit Change](#gerrit-build)
    - [Explicit Build](#gerrit-build-explicit)
    - [Triggered Build](#gerrit-build-triggered)
      - [Trigger Build by setting _Ready-for-Build_ on a change](#gerrit-build-trigger-vote)
      - [View changes used in the build](#gerrit-build-trigger-view)
      - [Post-Build](#gerrit-build-trigger-after)
  - [Test Gerrit Change(s)](#gerrit-test)


[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::

<hr>

This document contains information relating to the Android Workload jobs and how they can be used. While some of these training materials are Android-specific, some contain information that is also applicable to other workload areas (e.g. [gerrit](#gerrit)).

> [!IMPORTANT]
> - The URLs referenced in the instructions reference an example domain `example.horizon-sdv.com`; replace these URLs with your domain
> - Jenkins Dashboard: https://example.horizon-sdv.com/jenkins/

## <span style="color:#335bff">Prerequisites<a name="prerequisites"></a></span>

1. **Platform deployed** — Horizon SDV cluster and services (including Jenkins) are available for your environment; see your deployment guide.
2. **Workload Setup** — setup completed as per [workload_setup.md](workload_setup.md).
3. **Developer tools** — As required by the workloads you use (for example `git`, Google Cloud CLI, `adb` / `fastboot` for Android device flows).

[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::
## <span style="color:#335bff">ANDROID BUILDS</span> <a name="builds"></a>

The Horizon SDV platform inherently supports building using public manifests so any version of Android can be built using the Google AOSP upstream manifest for example. Additionally, if a private repo has been set up in Gerrit, this can also be used to build from a custom manifest.

[ --- Collapsing Section --- ]::
<details><summary>Source Selection in Build Job</summary><hr width="50%">

| Source | Build Job Parameters <sup>*</sup>  | 
| --- | --- |
| Google AOSP | `AAOS_GERRIT_MANIFEST_URL` = `https://android.googlesource.com/platform/manifest` <br> `AAOS_REVISION` = any branch present in the AOSP repo. E.g. `android-14.0.0_r30` |
| Gerrit Repo | `AAOS_GERRIT_MANIFEST_URL` = `https://example.horizon-sdv.com/gerrit/android/platform/manifest` <br> `AAOS_REVISION` = any branch present in the gerrit repo. E.g. `horizon/android-14.0.0_r30` |

<sup>*</sup> _Example build job_: _Android Workflows_ → _Builds_ → _AAOS Builder_

<hr width="50%"></details>

Documentation for the Android build pipelines is located at [docs/workloads/android/builds/](../android/builds/).


[ ========================== ]::
### <span style="color:#335bff">Building for Various Devices</span> <a name="build-devices"></a>

[ --- Collapsing Section --- ]::
<details id="build-devices-supported"><summary><b>Supported Devices (&amp; associated values)</b></summary><hr width="50%">
The following devices types are currently supported:

| Device Type | Target Prefix | Artifacts | Description |
| --- | --- | --- | --- |
| Android Studio Virtual Device | `sdk_car_` | `sdk-repo-linux-system-images.zip`<br>`horizon-sdv-aaos-sys-img2-1.xml` | Android SDK AVD targets can be used to spin up virtual devices using the _Android Emulator_ in _Android Studio_.|
| Cuttlefish Virtual Device | `aosp_cf_` | `cvd-host_package.tar.gz`<br>`aosp_cf_*_auto-img-builder.zip` | [Android Cuttlefish](https://source.android.com/docs/devices/cuttlefish) targets can be used to spin up virtual devices on virtual machine instances using the Cuttlefish host platform. <br>- the artifacts do not need to be downloaded for test pipeline jobs - simply note the google storage location | 
| Google Pixel Tablet | `tangorpro_car_` | `*out_sdv-aosp_tangorpro_car-*-userdebug.tgz*` | Android `tagoropro_car` system images can be used to flash [Google Pixel](https://source.android.com/docs/automotive/start/pixelxl) tablet devices. |
| Raspberry Pi | `aosp_rpi` | `*.img*`<br>e.g. `boot.img`, `system.img`, `vendor.img` | Android `aosp_rpi` system images can be used to flash Raspbery Pi devices. |

See the valid [Build Targets](#build-targets) options (in the _Other Build Topics_ section) for each device type/architecture/android version combination.

<hr width="50%"></details>

No matter which device is being built, the steps are the same:

[ --- Collapsing Section --- ]::
<details id="build-device-target"><summary><b>1. Build the Target:</b></summary><hr width="50%">

- Navigate to _AAOS Builder_ pipeline job (_Android Workflows_ → _Builds_ → _AAOS Builder_)
- Select `Build with Parameters`
- Use the gerrit manifest url provided as default
    - e.g. `https://example.horizon-sdv.com/gerrit/android/platform/manifest`
- Enter the required target for `AAOS_LUNCH_TARGET` (determines what type of build will take place, and which images are created)
    - See the valid [Build Targets](#build-targets) options (in the _Other Build Topics_ section) for each device type/architecture/android version combination
- Enter the required branch for `AAOS_REVISION` 
    e.g. `horizon/android-16.0.0_r3`
- Click `Build`

<hr width="50%"></details>

[ --- Collapsing Section --- ]::
<details id="build-get-artifacts"><summary><b>2. Download / Note the Build Artifacts</b></summary><hr width="50%">

- Wait for build to complete successfully (green tick in _Builds_ summary section)
- Open the artifacts text file (e.g. `*-userdebug-artifacts.txt`) which will indicate the location of the build artifacts.
- Download the artifacts if required (use the download commands in the artifacts file); otherwise, note the storage location.

<hr width="50%"></details>


[ ========================== ]::
### <span style="color:#335bff">Build the CTS Test Suite</span> <a name="build-cts"></a>

By default, the [CTS Execution](../android/tests/cts_execution.md) test job uses the Android CTS versions [provided by Google](https://source.android.com/docs/compatibility/cts/downloads), which are pre-installed on the VM instances used to launch Cuttlefish Virtual Devices. 

However, users can also build their own version of this test suite and use that instead.

- To build the CTS test suite, [Build the Target](#build-device-target) for a cuttlefish device, but with `AAOS_BUILD_CTS` parameter set to true; this builds only the test suite, not the target images.

- The output file does not need to be downloaded - simply note the google storage location (e.g. `gs://<BUCKET_ID>/Android/Builds/AAOS_Builder/03/android-cts.zip`); it can be used directly in the [CTS Execution](../android/tests/cts_execution.md) test job (instructions [here](#testing-cts-custom)).




[ ========================== ]::
### <span style="color:#335bff">Other Build Topics</span> <a name="build-other"></a>



[ --- Collapsing Section --- ]::
<details id="build-targets"><summary>Build Targets</summary><hr width="50%">

Android lunch targets are specified by the build parameter [`AAOS_LUNCH_TARGET`](../android/builds/aaos_builder.md#targets); they specify not only the device type, but also the android version, device architecture and required build variant: `<TargetPrefix_TargetArchitecture_AndroidReleaseIdentifier_BuildVariant>`

**Target Prefix** - Builds determine their functionality (e.g. setup, build args, archives) based on the LUNCH target prefix which changes by target type.

| Target Prefix | Device Type |
| --- | --- |
| `sdk_car*` | SDK Virtual Device targets |
| `aosp_cf*` | Cuttlefish Virtual Device targets |
| `*tangorpro_car*` | Pixel Tablet platform support |
| `aosp_rpi*` | Raspberry Pi targets (based on Vanilla RPi builds) |

- Unsupported target types provided to build jobs will be built using the default make command `m` (can be [overridden](../android/builds/aaos_builder.md#environment-variables) using the `OVERRIDE_MAKE_COMMAND` build parameter); however, no artifacts will be stored because the expected output files are not defined.

**Target Architecture** - Whether the target device is `arm` or `x86` also needs to be taken into consideration when the lunch target is specified.

**Android Release Identifier** - The device binaries used are version dependent - determined by the _release identifier_ (or _build release string_) - e.g. `ap1a`, `ap2a`, `ap3a`, `bp1a`, `bp3a`, `bp4a`. Currently-supported values are as follows:

| Android Version | Device Binary | Recommended for Use |
| --- | --- | --- |
| android-14.0.0_r30 | `ap1a` | yes |
| android-15.0.0_r4 | `ap3a` | no |
| android-15.0.0_r20 | `bp1a` | no |
| android-15.0.0_r32 | `bp1a` | no |
| android-15.0.0_r36 | `bp1a` | yes |
| android-16.0.0_r3 | `bp3a` | yes |
| android-16.0.0_r4 | `bp4a` | no |

This page lists the [Codenames, tags, and build IDs](https://source.android.com/docs/setup/reference/build-numbers) associated with each version of Android; to determine which _release identifier_ to use, inspect the _Build ID_ associated with the selected android version.

**Example Lunch Target**
The full list of Android targets supported on this platform for building Android cuttlefish, virtual devices, Pixel and RPi targets are as follows:

- Android Studio Virtual Device
    -   `sdk_car_x86_64-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `sdk_car_x86_64-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `sdk_car_x86_64-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `sdk_car_arm64-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `sdk_car_arm64-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `sdk_car_arm64-bp3a-userdebug` (`android-16.0.0_r3`)
- Cuttlefish Virtual Device
    -   `aosp_cf_x86_64_auto-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_cf_x86_64_auto-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_cf_x86_64_auto-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `aosp_cf_arm64_auto-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_cf_arm64_auto-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_cf_arm64_auto-bp3a-userdebug` (`android-16.0.0_r3`)
-   Pixel Devices:
    -   `aosp_tangorpro_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_tangorpro_car-bp1a-userdebug` (`android-15.0.0_r36` )
-   Raspberry Pi:
    -   `aosp_rpi4_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_rpi5_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_rpi4_car-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_rpi5_car-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_rpi4_car-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `aosp_rpi5_car-bp3a-userdebug` (`android-16.0.0_r3`)

<hr width="50%"></details>



[ --- Collapsing Section --- ]::
<details id="build-make"><summary>Over-riding Make Commands in Builds</summary><hr width="50%">

The Android Build jobs allow users to override the default make commands using the parameter `OVERRIDE_MAKE_COMMAND` (more info [here](../android/builds/aaos_builder.md#environment-variables)) for scenarios where the default is not sufficient.
- Example: when updates to the HAL have been made: `m android.hardware.automotive.vehicle.property-update-api && m dist`

See [this](android_labs.md#override-make) lab exercise which shows an example of its use.

<hr width="50%"></details>

[ --- Collapsing Section --- ]::
<details id="build-gerrit-change"><summary>Build a Gerrit Change</summary><hr width="50%">

For information on how to build a Gerrit change, refer to [this](#gerrit-build) part of the Gerrit section.

<hr width="50%"></details>


[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::
## <span style="color:#335bff">ANDROID DEVICES</span> <a name="devices"></a>

**Prerequisites:** The build artifacts required for each type of device have already been created and have either been saved locally or their location copied. See [Builds](#builds) section for more info, including which artifacts are required for each device type.

[ ========================== ]::
### <span style="color:#335bff">Android Studio Virtual Device</span> <a name="devices-android-studio"></a>

Android SDK AVD targets can be used to spin up virtual devices using the _Android Emulator_ in _Android Studio_.

[ --- Collapsing Section --- ]::
<details id="devices-android-studio-install"><summary><b>Install the Virtual Device</b></summary><hr width="50%">


>[!NOTE] if you have previously installed an older version of this device, [uninstall](#devices-android-studio-uninstall) the older version before proceeding.

- Launch Android Studio and Open the SDK Manager:
  - If you don’t have a project at this time, select the three vertical dots on Top Right Hand side from the `Welcome To Android Studio` menu and select `SDK Manager`.
  - If you have a project, open, then select from Top Level `Tools` → `SDK Manager`
    - Alternatively open `SDK Manager` from the Settings menu
- From the options on the left, select `Languages & Frameworks` → `Android SDK` and then select the `SDK Update Sites` tab.
- Select `+` to add your virtual device images/addon files
    - Locate your addon file `horizon-sdv-aaos-sys-img2-1.xml` and define the URL using `file:///`
    <br>e.g.`file:///Users/me/horizon-sdv/horizon-sdv-aaos-sys-img2-1.xml`
    - Select `OK` 
- In `Settings`, select `Apply`

- Install the package
    - From `Languages & Frameworks` → `Android SDK` → `SDK Platforms` select `Show Package Details`.
    - Find the reference to your image in the list:
        - look for an entry that starts with `Horizon SDV AAOS - Android/Builds/AAOS Builder-<build number>` - name is derived from the build that created it, so build number will match the Jenkins build job number.
    - Select `OK` and in `Confirm Change` select `OK` to install.
    - Wait for `SDK Component Installer` to complete 
    - Select `Finish`

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="devices-android-studio-launch"><summary><b>Launch the Virtual Device</b></summary><hr width="50%">

- Open the Device Manager:
  - If you don’t have a project at this time, select the three vertical dots on Top Right Hand side from the `Welcome To Android Studio` menu and select `Virtual Device Manager`.
  - If you have a project, open, then select from Top Level `Tools` → `Device Manager`.
- Select `+` sign and `Create Virtual Device` and in `Select Hardware`, select `Automotive` → `Automotive (1024p landscape)` and select `Next`.
- `System Image` should show the desired target image; select it and click on `Next`
  - If you wish, change the `AVD Name` within `Verify Configuration`.
- The Virtual Device is now available to use
- User may now run the device (_play_ button) and should see their device boot

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="devices-android-studio-uninstall"><summary><b>Uninstall the Virtual Device</b></summary><hr width="50%">

If you re-build your device image and wish to re-install the updated version, it is necessary to uninstall the previously-installed device first.

- `Android Studio` → `Virtual Device Manager` → 3 dots on device row → `Delete` → `Confirm`
- `Android Studio` → `SDK Manager` → `Languages & Frameworks` → `Android SDK` → `SDK Platforms` → untick the previous SDK you installed → `Apply`
- `Android Studio` → `SDK Manager` → `Languages & Frameworks` → `Android SDK` → `SDK Update Sites` → tick the previous entry you made → `minus` sign → `Ok`

<hr width="50%"></details>


[ ========================== ]::
### <span style="color:#335bff">Cuttlefish Virtual Device</span> <a name="devices-cuttlefish"></a>

[Android Cuttlefish](https://source.android.com/docs/devices/cuttlefish) targets can be used to spin up virtual devices on virtual machine instances using the Cuttlefish host platform.

[ --- Collapsing Section --- ]::
<details id="devices-cuttlefish-launch"><summary><b>Launch the Virtual Device</b></summary><hr width="50%">

- Navigate to _CVD Launcher_ pipeline job (`Android Workflows` → `Tests` → `CVD Launcher`)
- Select `Build with Parameters`
- Set `JENKINS_GCE_CLOUD_LABEL` = name of cloud to link to appropriate Instance Template (needs to match architecture)
  - Available clouds can be viewed in the Jenkins UI: `Settings` &rarr; `Clouds`.
- Set `CUTTLEFISH_DOWNLOAD_URL` = location of the cuttlefish target images 
    - e.g. `gs://<BUCKET_ID>/Android/Builds/AAOS_Builder/25`
    - Note that `CUTTLEFISH_DOWNLOAD_URL` is a Google Storage (gs) path and not a file or https url.
- Enable the `CUTTLEFISH_INSTALL_WIFI` option
- Set `CUTTLEFISH_KEEP_ALIVE_TIME` to the time you need to keep the virtual devices alive for
- Set `NUM_INSTANCES` to the number of devices you want to spin up; these will run in parallel and can be used to spread / shard testing across devices to optimise test run time.
- Wait for job to enter the `Keep Devices Alive` stage; the cuttlefish virtual devices are now alive and ready to be used. 

The virtual devices can be viewed and interacted with in _MTK Connect_ (_https://example.horizon-sdv.com/mtk-connect/docs/_); see MTK Connect usage instructions [here](workload_usage.md#appendix-mtk-connect-testbench-access).

Read more info on the [CVD Launcher](../android/tests/cvd_launcher.md) job.

<hr width="50%"></details>


[ ========================== ]::
### <span style="color:#335bff">Pixel Tablet Device</span> <a name="devices-pixel"></a>

See here for more info on [Pixel devices as development platrforms](https://source.android.com/docs/automotive/start/pixelxl).

>[!IMPORTANT] Unlocking the bootloader **_erases all data_** on the device.

**Prerequisites:** 
- `adb` and `fastboot` installed on local PC.
- Charging cable should be unplugged - if plugged in, see [tip](#pixel-tips)

[ --- Collapsing Section --- ]::
<details id="devices-pixel-unlock"><summary><b>Unlock Device</b></summary><hr width="50%">

- Enable **Developer options** from `Settings > System > About` and then tap `Build Number` seven times.
- Enable **USB debugging** and **OEM unlocking** from `Settings > System > Developer options`

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="devices-pixel-flash"><summary><b>Flash Device</b></summary><hr width="50%">

- Unpack the artifacts on your machine:
  - e.g. `tar -zxf out_sdv-aosp_tangorpro_car-bp1a-userdebug.tgz`
- Navigate to the location of the unpacked tar file
- Define `ANDROID_PRODUCT_OUT` so that `fastboot` can detect the `fastboot-info.txt` file and images to flash. (We do not have `LUNCH` target nor the full `OUT_DIR` hence we define the environment variable for `fastboot`.)
    - e.g. `export ANDROID_PRODUCT_OUT=out_sdv-aosp_tangorpro_car-bp1a-userdebug/target/product/tangorpro`
    - If a windows user and not using [WSL](https://learn.microsoft.com/en-us/windows/wsl/) then replace `export` with `set`.

- Place the device into fastboot mode and then unlock it
```
adb reboot bootloader
fastboot flashing unlock
```
- On the device, select `Unlock the Bootloader`. Doing so **_ERASES ALL DATA_** on the device!

>[!NOTE]
> - The `adb reboot bootloader` command initiates a reboot of the Android device, specifying that the device should enter the bootloader mode upon restarting.
>
> **Key Combination:** Most devices require a specific key combination (e.g., pressing volume down while the device boots) to enter the bootloader mode. This combination needs to be pressed during the reboot process triggered by the `adb reboot bootloader` command.
>
> **Bootloader Activation:** The physical interaction with the device (pressing the key combination) is what actually forces the device to boot into the bootloader instead of its normal OS.

- To flash the build:
```
fastboot -w flashall
```
- After the build starts booting with animation:
- Enable adb remount:
    ```
    # Temporary disable the userdata checkpoint
    adb wait-for-device root; sleep 3; adb shell vdc checkpoint commitChanges; sleep 2
    # Enable remount
    adb remount && sleep 2 && adb reboot && echo "rebooting the device" && adb wait-for-device root && sleep 5 && adb remount
    ```
- Push the required Automotive-specific files to the device:
    ```
    adb sync vendor && adb reboot
    ```
- Wait for the device to boot to the home screen

- User may now setup networking on the device and play.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="pixel-tips"><summary><b>Tips</b></summary><hr width="50%">

  - If you see screen brightness too low:
    ```
    adb shell settings put system screen_brightness 255
    ```
  - Boot when charger is plugged in:
    ```
    adb reboot bootloader
    fastboot oem off-mode-charge 1
    fastboot reboot
    ```
  - Enable Mock location:
    ```
    adb unroot
    adb shell cmd location set-location-enabled true
    adb root
    adb shell appops set 0 android:mock_location allow
    adb shell cmd location providers add-test-provider gps
    adb shell cmd location providers set-test-provider-enabled gps true
    adb shell cmd location providers set-test-provider-location gps --location 37.090200,-95.712900
    #To verify
    adb shell dumpsys location | grep "last location"
    ```
- An APK can be installed using `adb install <APK FILENAME>.apk`.


<hr width="50%"></details>

[ ========================== ]::
### <span style="color:#335bff">Raspberrry Pi Device</span> <a name="devices-rpi"></a>

Android `aosp_rpi` system images can be used to flash Raspbery Pi devices.

[ --- Collapsing Section --- ]::
<details id="devices-rpi-create-image"><summary><b>Create a Flashable Image</b></summary><hr width="50%">

- Download the folowing bash scripts which will be used to create the flashable image - select the appropriate one for the RPi version you are targeting:
  - RPi5 [mkimg.sh](https://github.com/raspberry-vanilla/android_device_brcm_rpi5/blob/android-16.0/mkimg.sh) 
  - RPi4 [mkimg.sh](https://github.com/raspberry-vanilla/android_device_brcm_rpi4/blob/android-16.0/mkimg.sh)

- Run the script to create the flashable image:
  - it must be run on a host that supports loop devices (Horizon SDV build instances are Docker containers running in kubernetes and do not have the privileges to support loop devices). 
  Run as follows:
  ```
  # CHANGE ANDROID_PRODUCT_OUT <path to img files> with the path to the downloaded files
  # Change TARGET_PRODUCT to match the AAOS_LUNCH_TARGET
  TARGET_PRODUCT=aosp_rpi5_car-bp3a-userdebug \
  ANDROID_PRODUCT_OUT=<path to img files> \
  ./mkimg.sh
   ```
- The flashable image will be created. E.g. `RaspberryVanillaAOSP15-<date>-rpi5_car-bp3a-userdebug.img`.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="devices-rpi-flash"><summary><b>Flash the Device</b></summary><hr width="50%">

Use the [Raspberry Pi Imager](https://www.raspberrypi.com/software) to flash this image onto your device.


> [!NOTE]
> - User may wish to build from a different release, if so, use the Google AOSP manifest and updated versions, e.g.
>   - `AAOS_GERRIT_MANIFEST_URL` `https://android.googlesource.com/platform/manifest`
>   - `AAOS_REVISION` `android-15.0.0_r36`
>   - `AAOS_LUNCH_TARGET` `aosp_rpi5_car-bp1a-userdebug` or `aosp_rpi4_car-bp1a-userdebug`
>
> - Or build for Android 14:
>   - `AAOS_GERRIT_MANIFEST_URL` `https://android.googlesource.com/platform/manifest`
>   - `AAOS_REVISION` `android-14.0.0_r67`
>   - `AAOS_LUNCH_TARGET` `aosp_rpi5_car-ap2a-userdebug` or `aosp_rpi4_car-ap2a-userdebug`
>   - `POST_REPO_INITIALISE_COMMAND` `curl -o .repo/local_manifests/manifest_brcm_rpi.xml -L https://raw.githubusercontent.com/raspberry-vanilla/android_local_manifest/android-14.0/manifest_brcm_rpi.xml --create-dirs && curl -o .repo/local_manifests/remove_projects.xml -L https://raw.githubusercontent.com/raspberry-vanilla/android_local_manifest/android-14.0/remove_projects.xml`
>
> Also, Vanilla RPi updates move revisions frequently, so one week the builds may work and the next not. Keep up to date on their
> changes and use the parameters defined above to override.

> [!TIP]
> - `POST_REPO_INITIALISE_COMMAND` build parameter allows the user to override the post repo init commands, e.g. update the RPi manifest.
>   - Refer to OSS repository: [horizon-sdv](https://github.com/googlecloudplatform/horizon-sdv) `docs/workloads/android/builds/aaos_builder.md` and build scripts for more details.

<hr width="50%"></details>




[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::
## <span style="color:#335bff">ANDROID TESTING</span> <a name="testing"></a>

**Prerequisites:** The build artifacts required for each type of device have already been created and have either been saved locally or their location copied. See [Builds](#builds) section for more info, including which artifacts are required for each device type.

> [!TIP] <a name="test-job-tip"></a>
> Ensure that Test jobs ([CVD Launcher](../android/tests/cvd_launcher.md), [CTS Execution](../android/tests/cts_execution.md)) always set
> - an appropriate cloud label (_JENKINS_GCE_CLOUD_LABEL_) to link to an instance template of the correct architecture (view available clouds in the Jenkins UI: `Settings` &rarr; `Clouds`).
> - the correct build artifacts (_CUTTLEFISH_DOWNLOAD_URL_), built for the desired device / android version / architecture
> - the correct _ANDROID_VERSION_ for the selected build artifacts

[ ============================================================================= ]::
### <span style="color:#335bff">Automated Testing - CTS</span> <a name="testing-cts"></a>

[ --- Collapsing Section --- ]::
<details id="testing-cts-default"><summary>Run Default CTS on Cuttlefish Virtual Device(s)</summary><hr 

- Navigate to _CTS Execution_ pipeline job (`Android Workflows` → `Tests` → `CTS Execution`)
- Select `Build with Parameters` 
- Set `JENKINS_GCE_CLOUD_LABEL` = name of approprate Instance Template (needs to match architecture)
  - Available clouds can be viewed in the Jenkins UI: `Settings` &rarr; `Clouds`.
- Set `CUTTLEFISH_DOWNLOAD_URL` = location of the cuttlefish target images 
    - e.g. `gs://<BUCKET_ID>/Android/Builds/AAOS_Builder/25`
    - Note that `CUTTLEFISH_DOWNLOAD_URL` is a Google Storage (gs) path and not a file or https url.
- Enable the `CUTTLEFISH_INSTALL_WIFI` option if required
- Set `ANDROID_VERSION` to the version of Android that was used to build the target image specified in `CUTTLEFISH_DOWNLOAD_URL`; this will be used to select the appropriate test harness
- Set `NUM_INSTANCES` to the number of devices you want to spin up; these will run in parallel and can be used to spread / shard testing across devices to optimise test run time. Sharding will be done automatically if more than 1 device is selected.
- Set the CTS Parameters `CTS_TESTPLAN` and `CTS_MODULE` (see [here](../android/tests/cts_execution.md#environment-variables) for more info):
    - Set `CTS_TESTPLAN` to whichever value is appropriate for the Android version being used
    - Leave `CTS_MODULE` empty to run all modules, or set to an individual module (e.g. `CtsDeqpTestCases`) if desired. 
- Select `Build`

Read more info on the [CTS Execution](../android/tests/cts_execution.md) job and read [tips](#testing-cts-tips) below.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="testing-cts-custom"><summary>Run Custom CTS on Cuttlefish Virtual Device(s)</summary><hr 

**Prerequisite:** Custom CTS test suite was already built and download location is known. See 
 [build](#build-cts) section for more info and instructions.

To perform testing using a custom-built CTS, follow the same proceedure as for the [default CTS run](#testing-cts-default) but in addition:
- Set `CTS_DOWNLOAD_URL` to point to your prebuilt CTS (e.g. `gs://<BUCKET_ID>/Android/Builds/AAOS_Builder/01/android-cts.zip`)
    - Note: ensure the full URL including `android-cts.zip` is used

This job will take longer than a job using the default CTS because it needs to download and unpack the custom `android-cts.zip` from Google Cloud Storage.

All artifacts are as per runs using the default CTS.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="testing-cts-artifacts"><summary>CTS Artifacts</summary><hr 

Wait for build to complete successfully (green tick in _Builds_ summary section)

The test artifacts are stored in the Jenkins job - e.g. details of the CTS Modules, Test Plans, Results etc:

Summary of artifacts (not an extensive list):
    - `cts-modules.txt` shows the available modules for `CTS_MODULE` parameter
    - `cts-plans.txt` shows the available plans for `CTS_TESTPLAN` parameter
    - `invocation_summary.txt` shows the test result summary
    - The `*.zip` file contains the full set of results files.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="testing-cts-tips"><summary>CTS Tips</summary><hr 

- A single module can be run when getting familiar with testing pipeline as the full test suite may take hours to complete.

- When multiple devices are used, the testing is spread over devices on a module-by-module basis, so if you want to see multiple devices being used for testing you will need to run a full CTS test plan (i.e. leave the `CTS MODULE` parameter empty)

- Running the pipeline with the parameter `CTS_TEST_LISTS_ONLY` ticked will skip the testing and simply generate a list of the test plan and test modules; this can be useful when selecting a test plan to execute. These lists are also generated by default as part of a full test run.

- Running the pipeline with the parameter `MTK_CONNECT_ENABLE` ticked will allow the user to view the devices during the testing process. However, interaction with the devices should be avoided as this would interfere with the testing process.

<hr width="50%"></details>

[ ============================================================================= ]::
### <span style="color:#335bff">Manual Testing - MTK Connect with Cuttlefish Device</span> <a name="testing-mtk-connect-cuttlefish"></a>

Refer to the [Cuttlefish Virtual Device](#devices-cuttlefish) part of the 'Devices' section for instructions on how to launch Cuttlefish virtual device(s).

To test the virtual device(s) manually using _MTK Connect_ ('https://example.horizon-sdv.com/mtk-connect/docs/') see the instructions here: [MTK Connect Testbench Access](workload_usage.md#appendix-mtk-connect-testbench-access).


[ ============================================================================= ]::
### <span style="color:#335bff">Manual Testing - Android Studio</span> <a name="testing-android-studio"></a>

Refer to the [Android Studio Device](#devices-android-studio) part of the 'Devices' section for instructions on how to launch a virtual device in _Android Studio_.

Alternatively, a [Cuttlefish device](#devices-cuttlefish) can be launched and accessed in _Android Studio_ via an MTK Connect tunnel (see [here](workload_usage.md#appendix-mtk-connect-testbench-access-tunnel) for instructions).

[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::
## <span style="color:#335bff">ANDROID CHANGE USING GERRIT</span> <a name="gerrit"></a>

> [!NOTE]
> - All git operations here should be performed as per usual methods (e.g. in WSL for Windows users) - it is not intended that this be done in Android Studio.
> - URLs differ per project, so DO NOT CUT AND PASTE from this text - copy from Gerrit only.
> - Your Horizon SDV Gerrit credentials and HTTP token/password as created during [Gerrit access setup](workload_setup.md#access-gerrit) will need to be used.
> - To set up a new Gerrit repo/project refer to [this](workload_setup.md#gerrit-setup) setup guide.

[ ========================== ]::
### <span style="color:#335bff">Creating a Gerrit Change for Review</span> <a name="gerrit-change-review"></a>

**AIM:** Make a code change to a gerrit repo and stage the change in Gerrit Code Review.

[ --- Collapsing Section --- ]::
<details id="gerrit-change-clone"><summary><b>Clone Gerrit Repo</b></summary><hr width="50%">

- In Gerrit select `BROWSE` → `Repositories` → select the required repo (e.g.`android/platform/packages/apps/Car/Launcher`)
- Copy the `Clone with commit-msg hook`
- Paste the copied command to clone the repo (Note: use the copied command, not the example given here):
    <pre>git clone "https://example.horizon-sdv.com/gerrit/android/platform/packages/apps/Car/Launcher" && (cd "Launcher" && mkdir -p `git rev-parse --git-dir`/hooks/ && curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://example.horizon-sdv.com/gerrit/tools/hooks/commit-msg && chmod +x `git rev-parse --git-dir`/hooks/commit-msg)</pre>


<hr width="50%"></details>

[ --- Collapsing Section --- ]::
<details id="gerrit-change-edit"><summary><b>Modify the Code</b></summary><hr width="50%">

- Navigate to the cloned folder 
    - e.g. `cd Launcher`
- Identify the current branch
    - `git branch --show-current`
- Switch to another branch, if required
    - e.g. `git checkout horizon/android-16.0.0_r3`
- Open the desired file(s) 
    - e.g. `app/res/values/strings.xml`
- Make the required change(s)
    - e.g. change the value given for `weather_app_name` :
    `<string name="weather_app_name">Horizon-SDV Weather</string>`
- Save the file(s)

<hr width="50%"></details>

[ --- Collapsing Section --- ]::
<details id="gerrit-change-push"><summary><b>Push Changes for Review</b></summary><hr width="50%">

  - Commit: 
    - e.g. `git commit -am "Car Launcher weather app - sample update"`

  - Update Change ID if one was not automatically generated in your commit: 
    - `git commit --amend --no-edit`

  - Push for Review to the current branch:
    - The push can be done either with or without [_Topic_](#gerrit-change-id-topic) info
    - **Without a _Topic_** 
        - `git push origin HEAD:refs/for/$(git branch --show-current)`
    - **With a _Topic_**
        - `git push origin HEAD:refs/for/$(git branch --show-current)%topic=<topic_name>`
    - The remote should report success and provide a link back to the Gerrit review, e.g. `https://example.horizon-sdv.com/gerrit/c/android/platform/packages/apps/Car/Launcher/+/12`

<hr width="50%"></details>


[ ========================== ]::
### <span style="color:#335bff">Gerrit Change Identification</span> <a name="gerrit-change-id"></a>

[ --- Collapsing Section --- ]::
<details id="gerrit-change-id-single"><summary><b>Using Single Change Parameters</b></summary><hr width="50%">

A single Gerrit change is identified by 3 parameters which can all be seen on the Gerrit page which shows your particular change. Select `CHANGES` → `Open` → select your change (e.g.`Car Launcher weather app - sample update`).

| Change Parameter | Where to find it |
| --- | --- |
| `Project` | hover on the 'Repo' link in the Properties panel |
| `Change Number` | top left corner of page - before the commit message |
| `Patchset Number`| In the panel showing 'Files', 'Comments' - e.g. `Patchset 1` |

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="gerrit-change-id-topic"><summary><b>Using _Topic_ Property</b></summary><hr width="50%">

Changes can be identified in Gerrit using the `Topic` property. This is particularly useful for grouping multiple changes together, although it is still a valid identificaiton method for single changes.

Being able to group changes allows developers to test changes that span multiple repositories, ensuring that the changes can be tested together.

**Topic Assignment:**

- **Explicit Assignment on Change Page**
    - Select the 'edit' symbol next to the `Topic` field in the Properties panel.
    - Enter the desired topic value and click `SET TOPIC`
    
- **Assignment During Push Operation**
    - While pushing the change for review, append `%topic=<topic_name>` to the ref path:
        - e.g. `git push origin HEAD:refs/for/$(git branch --show-current)%topic=<topic_name>`

Regardless of the method of assignment, the new _Topic_ value should now show on the change page in Gerrit:

>[!NOTE:] 
> - A _Topic_ cannot be assigned to multiple changes simultaneously - it needs to be assigned to each individually
>- Click on the _Topic_ name to see all changes marked with the same _Topic_ 

<hr width="50%"></details>

[ ========================== ]::
### <span style="color:#335bff">Build with Gerrit Change</span> <a name="gerrit-build"></a>

For general build instructions refer to the [builds](#builds) section.

[ --- Collapsing Section --- ]::
<details id="gerrit-build-explicit"><summary><b>Explicit Build</b></summary><hr width="50%">

The stardard build pipeline _AAOS Builder_ can be used to build using a Gerrit change or group of Gerrit changes for a particular target.

Specific changes can be selected either individually or as a group using the following Build job [Parameters](../android/builds/aaos_builder.md#environment-variables): 

| Parameter(s) | Single Change | Multiple Changes | Where to find values |
| --- | --- | --- | --- |
|  `GERRIT_PROJECT`<br>`GERRIT_CHANGE_NUMBER`<br>`GERRIT_PATCHSET_NUMBER` | Yes | No | [single change params](#gerrit-change-id-single) |
|  `GERRIT_TOPIC` | Yes | Yes | [topic](#gerrit-change-id-topic) |

Build artifacts will be generated when a build is successful; the location of the system images for the specified target is given in the `-artifacts.txt` file shown in the job artifacts list.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="gerrit-build-triggered"><summary><b>Triggered Build</b></summary><hr width="50%">

The [_Gerrit_](../android/builds/gerrit.md) build pipeline can be used to test a Gerrit change or group of Gerrit changes; it builds for a fixed set of targets, spins up a virtual cuttlefish device and runs a basic CTS test run.

This job is triggered whenever the `Ready-for-Build` flag is set to +1 on a Gerrit patchset change; if the change is part of a _Topic_, all changes within that topic will be pulled into the triggered build.

**Recommended Reading:** How the _Gerrit_ job is [triggered](../android/builds/gerrit.md#gerrit-triggers).

[ --- Collapsing SubSection --- ]::
<details id="gerrit-build-trigger-vote"><summary><b>Trigger Build by setting <i>Ready-for-Build</i> on a change</b></summary><hr width="50%">

- Open the change in the Gerrit application
    - Select `CHANGES` → `Open` → select your change (e.g.`Car Launcher weather app - sample update`).
- Hover on the `Ready-for-Build` item in the _Submit Requirements_ panel, then click on `VOTE READY-FOR-BUILD`
    - If you don’t see the `Ready-to_Build` label, ensure the correct [access](workload_setup.md#gerrit-project-access) permissions have been set
- The `Ready-for-Build` flag should now be set 
- A Gerrit build should have triggered 
    - It will be in the quiet period initially - see [more info](../android/builds/gerrit.md#gerrit-triggers)
    - After the quiet period, the build begins and a _Build Started_ comment will be added to the _Change Log_ section of the Gerrit change page with a link to the triggered build job.    
    _Note: this will only appear for the change which triggered the build - not for the other changes in the topic_


<hr width="50%"></details>


[ --- Collapsing SubSection --- ]::
<details id="gerrit-build-trigger-view"><summary><b>View changes used in the build</b></summary><hr width="50%">

Open the Gerrit job in Jenkins (find the job manually or use the link provided in the _Change Log_ section of the change page).
- The _Console Log_ will show the details of the change on which the `Ready-for-Build` flag was set (including the _Topic_ info, if present)
- Importantly, if the change which triggered the build had a _Topic_ set, the start of each build stage will also show that all changes within the _Topic_ are fetched - not just the change which triggered the build.

<hr width="50%"></details>


[ --- Collapsing SubSection --- ]::
<details id="gerrit-build-trigger-after"><summary><b>Post-Build</b></summary><hr width="50%">

Build should run and if successful, each change in the _Topic_ (or a single change if no topic was set) will receive a `VERIFIED` label vote as to whether successful.
- The status of each build target will also be added to the _Change Log_ section of the Gerrit change page

Build artifacts will be generated for each target that was built successfully; the location of the system images for each target is given in the `-artifacts.txt` files shown in the job artifacts.

<hr width="50%"></details>

<hr width="50%"></details>


[ ========================== ]::
### <span style="color:#335bff">Test Gerrit Change(s)</span> <a name="gerrit-test"></a>

Once a build has been executed and target images created, the desired target device can be launched (for a virtual device) or flashed (for a real device) in order to test the change that was made. 

E.g. For the example change made [here](#gerrit-change-edit) the name of the weather app would be expected to change - whether the change is 
- on a [virtual device in Android Studio](#devices-android-studio)
- on a [cuttlefish device](#devices-cuttlefish) viewed via [MTK Connect](workload_usage.md#appendix-mtk-connect-testbench-access) 
- flashed onto a [real pixel device](#devices-pixel).

