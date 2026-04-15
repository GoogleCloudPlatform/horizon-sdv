# Agentic PR Review & OSS Scanner

This directory contains tools for automated, AI-powered code reviews and dependency scanning for the Horizon SDV project.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [Components](#components)
- [GitHub Actions Integration](#github-actions-integration)
- [Configuration](#configuration)

## Overview

The *Agentic PR Review & OSS Scanner* system leverages the **Gemini 2.0 Flash** model to perform intelligent code reviews and dependency discovery. It is designed to enhance the development lifecycle by providing early feedback on security, code quality, and compliance directly within the CI/CD pipeline.

## Installation

To use these scripts locally for testing or development:

1.  **Navigate to the directory**:
    ```bash
    cd tools/scripts/
    ```

2.  **Install dependencies**:
    The scripts require `requests` and `urllib3`.
    ```bash
    pip install requests urllib3
    ```

## Usage

### Agentic Review
To run the code review script manually against the `main` branch:
```bash
export GEMINI_API_KEY="your_api_key_here"
python agentic_review.py main
```

### OSS Scan
To run the dependency scanner:
```bash
python oss_scan.py
```

## Components

### 1. `agentic_review.py`
This script performs an AI-driven review of the git diff between the current branch and a target base branch (defaulting to `main`).
- **Security Audit**: Scans for leaked secrets and insecure configurations.
- **Best Practices**: Enforces project standards (e.g., `set -e` in shell scripts).
- **Suggested Changes**: Gemini is instructed to provide fixes in GitHub "Suggested Change" format.
- **Resilience**: Includes built-in retry logic and diff truncation for large changes.

### 2. `oss_scan.py`
A discovery tool that identifies open-source dependency files across the repository.
- **Supported Ecosystems**: Python, Node.js, and Go.
- **Filtering**: Automatically ignores build artifacts and environment directories (`.terraform`, `node_modules`, etc.).

## GitHub Actions Integration

The system is fully integrated via `.github/workflows/agentic-pr-review.yml`.

- **Triggers**: Runs on every `push` to any branch and every `pull_request` targeting `main`.
- **Dynamic Commenting**: 
    - For Pull Requests: Posts feedback as a PR comment.
    - For Direct Pushes: Posts feedback as a commit comment.
- **Performance**: Utilizes GitHub Actions caching for Python dependencies and implements concurrency controls to manage workflow runs.

## Configuration

The system can be customized using GitHub Secrets and Variables:

- `GEMINI_API_KEY` (Secret): **Required.** Your Google AI Studio API key.
- `GEMINI_MODEL` (Variable): **Optional.** Defaults to `gemini-2.0-flash`. Can be set to `gemini-1.5-pro` for deeper analysis.

---
*Note: This tool is intended to assist human reviewers and should not be the sole basis for security or architectural decisions.*
