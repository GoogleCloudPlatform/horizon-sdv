# Standalone ABFS Client Deployment Guide (GCE VM)

This guide provides a concise workflow to provision an Ubuntu 24.04 VM as an ABFS client, pin the Linux kernel to `6.14.0-1021-gcp`, mount an AOSP manifest (`android-16.0.0_r4`), and trigger an Android build.

Pinning kernel **`6.14.0-1021-gcp`** ensures symbol compatibility with `casfs-kmod-6.14.0-1021-gcp` (version `0.1.14`) and `abfs-client` (`0.1.14`).

---

## 1. Step 1: Provision the Client VM

Assuming [STANDALONE_ABFS_DEPLOYMENT_GUIDE.md](STANDALONE_ABFS_DEPLOYMENT_GUIDE.md) was executed (`vpc-abfs`, `subnet-abfs`, firewall rules, and `abfs-runtime` service account exist), provision a high-memory VM inside `vpc-abfs`:

```bash
export PROJECT_ID="YOUR_PROJECT_ID"
export ZONE="europe-west3-a"
export INSTANCE_NAME="abfs-client-vm"

gcloud compute instances create ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --machine-type=n2-highmem-32 \
    --image-family=ubuntu-2404-lts-amd64 \
    --image-project=ubuntu-os-cloud \
    --boot-disk-type=pd-ssd \
    --boot-disk-size=500GB \
    --network=vpc-abfs \
    --subnet=subnet-abfs \
    --no-address \
    --scopes=cloud-platform \
    --service-account=abfs-runtime@${PROJECT_ID}.iam.gserviceaccount.com \
    --metadata=enable-oslogin=TRUE
```

> [!TIP]
> If `n2-highmem-32` is quota-constrained, substitute `--machine-type=n2-standard-32` or `--machine-type=e2-standard-32`.

---

## 2. Step 2: Automated Kernel Pinning & CASFS Installation

The script at [scripts/setup-abfs-vm.sh](../scripts/setup-abfs-vm.sh) automates APT repository setup, kernel pinning (`6.14.0-1021-gcp`), AppArmor relaxation, and `casfs.ko` module insertion.

Allow ~30 seconds after VM creation for SSH to initialize, then execute:

```bash
# 1. Copy setup-abfs-vm.sh to your VM via IAP
gcloud compute scp scripts/setup-abfs-vm.sh ${INSTANCE_NAME}:~/setup-abfs-vm.sh \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --tunnel-through-iap

# 2. SSH into the VM, make the script executable, and install pinned kernel & packages
gcloud compute ssh ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --tunnel-through-iap \
    --command="chmod +x ~/setup-abfs-vm.sh && sudo ~/setup-abfs-vm.sh"

# 3. Reboot the VM to boot into the pinned Linux kernel (6.14.0-1021-gcp)
gcloud compute ssh ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --tunnel-through-iap \
    --command="sudo reboot" || true

# 4. Allow ~30 seconds for reboot, then SSH in and run the script once more to insert casfs.ko
sleep 30
gcloud compute ssh ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --tunnel-through-iap \
    --command="sudo ~/setup-abfs-vm.sh"
```

> [!IMPORTANT]
> The second execution in step 4 loads `casfs.ko` (`version 0.1.14`) and verifies installation.

---

## 3. Step 3: Initialize ABFS & Mount AOSP

Once running on kernel `6.14.0-1021-gcp` with `casfs` loaded, connect to your VM and initialize the client:

```bash
# 1. SSH into your VM via IAP
gcloud compute ssh ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --tunnel-through-iap

# 2. Export the reachable address of your ABFS server (internal LB IP or DNS name).
#    To find this IP from your local GKE cluster terminal, run:
#        kubectl get svc abfs-server -n abfs -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
#    (e.g., export SERVER_ADDRESS="10.0.0.58")
export SERVER_ADDRESS="<YOUR_ABFS_SERVER_IP_OR_DNS>"

# 3. Configure ABFS client
abfs --remote-servers ${SERVER_ADDRESS}:50051 \
     --tunnel-ports 0 \
     --manifest-server android.googlesource.com \
     --manifest-project-name platform/manifest \
     --disable-tls=true \
     --auth-type none \
     config -w

# 4. Initialize local cache and configuration
abfs init

# 5. Perform health check against server
abfs test-client health-check

# 6. Start cacheman daemon in background
abfs cacheman run > /tmp/cacheman.log 2>&1 &
abfs cacheman ping -t 1m

# 7. Mount the full AOSP release manifest into ~/src
mkdir -p ~/src
abfs mount -b android-16.0.0_r4 ~/src
```

---

## 4. Step 4: Triggering an Android Build

Once `abfs mount` completes, navigate into `~/src` to compile Android:

```bash
cd ~/src

abfs setup $PWD

# Initialize the Android build environment
source build/envsetup.sh

# Select target (e.g., Cuttlefish x86_64 phone userdebug)
lunch aosp_cf_x86_64_phone-trunk_staging-userdebug

# Execute build
m droid
```

> [!TIP]
> **Running Persistent Builds (Surviving SSH Disconnection)**
> AOSP compilation can take an hour or more. To ensure your build continues running even if your laptop sleeps or your SSH session drops:
> * **Option A (Recommended — `tmux`)**:
>   ```bash
>   tmux new -s aosp               # 1. Start a named tmux session
>   # ... run envsetup, lunch, and m droid inside tmux ...
>   # 2. Detach anytime without killing the build: Press Ctrl+b, then d
>   tmux attach -t aosp            # 3. Reconnect to monitor progress anytime
>   ```
> * **Option B (`nohup` Background Build)**:
>   ```bash
>   nohup m droid > ~/src/build.log 2>&1 &
>   tail -f ~/src/build.log        # Check logs anytime
>   ```

---

## 5. Troubleshooting & Operations Reference

For debugging workflows - including checking `casfs.ko` registration (`lsmod | grep casfs`), kernel diagnostics (`dmesg`), tailing `cacheman` logs, and running `abfs doctor` - refer to [Section 8 of the ABFS Troubleshooting Guide](ABFS_TROUBLESHOOTING_GUIDE.md#8-standalone-gce-vm-troubleshooting--operations-reference).
