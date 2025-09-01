# Horizon

Welcome! 

This page provides an introduction to Horizon, a turnkey implementation designed for OEMs and suppliers interested in adopting Google Cloud’s Software-Defined Vehicle (SDV) industry solution.

The program addresses prevalent challenges within the SDV market, including the inefficiencies stemming from complex and inconsistent toolchains, the high scaling costs that impede software development, and a hardware-centric focus that constrains rapid development and innovation.

## Getting started

Horizon is intended to be deployed into a project within your Google Cloud tenant. The deployment is done leveraging Terraform and Argo CD.

To deploy Horizon into your project, follow the steps described in the [deployment guide](https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/deployment_guide.md). 

We would be happy to accept your contributions, for details refer to the [contribution guide](https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/contributing.md).

For consultation regarding initial setup, specific use case, technology or other themes, please contact horizon-sdv@google.com.

## Vision
Platforms enabling efficient software development shouldn’t be differentiating, the product itself should be differentiating.

Horizon provides an open-source, robust, scalable, cloud-hosted and developer friendly toolchain for the development of complex embedded software. It provides solutions for code, build, test and release.

By shifting the focus from hardware-led to software-centric development, it aims to drive faster innovation, reduce costs, and improve the quality of software development in the automotive industry.

## Initiative
The Horizon program is an initiative launched by Google and Accenture, poised to revolutionize the embedded software development landscape.

It addresses challenges in the Software-Defined Vehicle (SDV) market such as complex and inconsistent
toolchains leading to inefficiencies, high scaling costs that constrain software development and a
hardware-centric focus that hinders rapid development and innovation.

## Streams
The Horizon initiative consists of two streams: 
* Stream 1 is the current focus and covers Android Platform development (AAOS IVI, AOSP). 
* Stream 2 will focus on expanding beyond Android towards the rest of SDV, enabling cloud-based virtual development of complex system-of-systems.

## Platform overview
![Horizon Platform Overview](https://raw.githubusercontent.com/GoogleCloudPlatform/horizon-sdv/refs/heads/main/docs/images/horizon_platform_overview.svg)

## Major upcoming roadmap items
* [Cloud Workstations](https://cloud.google.com/workstations) directly deployable via Horizon (https://github.com/GoogleCloudPlatform/cloud-workstations-custom-image-examples/tree/main/examples/images/android-open-source-project)
* Android Build File System (ABFS) Server, Pushers and Clients directly deployable from Horizon and integrated with scalable GKE based build farm (https://github.com/terraform-google-modules/terraform-google-abfs)
* Virtual devices deployed via Cloud Android Orchestration (https://github.com/google/cloud-android-orchestration/) 


## Staying up-to-date
In case you would like to receive updates, you can [subscribe here](https://forms.gle/TFaKXqfHbF6oUAeg6). In case of questions you can directly reach out to [horizon-sdv@google.com](mailto:horizon-sdv@google.com)
For specific technical issues you can [raise an issue on Github](https://github.com/GoogleCloudPlatform/horizon-sdv/issues).

## Disclaimer
This is not an officially supported Google product. This project is not
eligible for the [Google Open Source Software Vulnerability Rewards
Program](https://bughunters.google.com/open-source-security).
