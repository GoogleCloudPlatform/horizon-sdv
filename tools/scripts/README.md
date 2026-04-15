# Agentic PR Review & OSS Scanner 🤖

This directory contains tools for automated, AI-powered code reviews and dependency scanning for the Horizon SDV project.

## 🌟 Overview

The system uses the **Gemini 2.0 Flash** model to perform intelligent code reviews on every pull request and push to the repository. It focuses on security, best practices, correctness, and maintainability, providing actionable feedback and direct code suggestions.

## 🛠️ Components

### 1. `agentic_review.py`
- **Function**: Performs an AI-driven review of the git diff between the current branch and `main`.
- **Features**:
    - **Intelligent Prompting**: Instructs Gemini to look for security leaks, shell script best practices (`set -e`), and Terraform modularity.
    - **Suggested Changes**: Requests Gemini to format fixes as GitHub "Suggested Change" blocks for easy application.
    - **Production Ready**: Includes API retry logic (urllib3), diff size limits (100k chars), and configurable models.
    - **Logging**: Detailed logging for debugging API interactions.

### 2. `oss_scan.py`
- **Function**: Scans the repository for common open-source dependency files.
- **Supported Ecosystems**: Python (`requirements.txt`, `Pipfile`, `pyproject.toml`), Node.js (`package.json`, etc.), and Go (`go.mod`, `go.sum`).
- **Features**: Excludes build/environment directories (`.terraform`, `node_modules`, `__pycache__`) and provides a structured markdown report.

### 3. `.github/workflows/agentic-pr-review.yml`
- **Function**: Orchestrates the review process in GitHub Actions.
- **Triggers**: Runs on every `push` to any branch and every `pull_request` targeting `main`.
- **Features**:
    - **Caching**: Caches Python dependencies (pip) to speed up runs.
    - **Concurrency**: Prevents multiple overlapping reviews on the same PR/branch.
    - **Dynamic Comments**: Automatically detects if it should post a comment to a Pull Request or a direct Commit.

## 🚀 Setup

To enable this system in your repository:

1.  **Obtain an API Key**: Get a Gemini API key from the [Google AI Studio](https://aistudio.google.com/).
2.  **Add GitHub Secret**: Go to your repository settings -> Secrets and variables -> Actions and add a new repository secret:
    - **Name**: `GEMINI_API_KEY`
    - **Value**: `<your-api-key>`
3.  **(Optional) Configure Model**: If you want to use a different model (e.g., `gemini-1.5-pro`), add a repository variable:
    - **Name**: `GEMINI_MODEL`
    - **Value**: `gemini-1.5-pro`

## 📝 Usage

Once configured, the system works automatically:
- **On Push**: Posts a review comment directly to the commit.
- **On Pull Request**: Posts a review comment to the PR thread.
- **Review Content**: Look for the "Agentic PR Review 🤖" and "OSS Dependency Scan 🔍" sections in the comments.

---
*Note: This tool is intended to assist human reviewers and should not be the sole basis for security or architectural decisions.*
