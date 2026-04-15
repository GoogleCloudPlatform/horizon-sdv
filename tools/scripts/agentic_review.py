#!/usr/bin/env python3
import os
import subprocess
import sys
import json
import requests

# This script performs an agentic code review using the Gemini API.
# It compares the current branch with 'main' and sends the diff to Gemini for analysis.
#
# Prerequisites:
# - GEMINI_API_KEY environment variable set.
# - 'git' installed and available in PATH.

GEMINI_API_URL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

def get_git_diff(base_branch="main"):
    """Gets the diff between the current state (including unstaged/untracked) and the base branch."""
    try:
        # Check if base branch exists
        subprocess.run(["git", "rev-parse", "--verify", base_branch], check=True, capture_output=True)
        
        # 1. Get staged/unstaged changes compared to base_branch
        diff_committed = subprocess.run(["git", "diff", f"{base_branch}...HEAD"], capture_output=True, text=True, check=True).stdout
        diff_unstaged = subprocess.run(["git", "diff", "HEAD"], capture_output=True, text=True, check=True).stdout
        
        # 2. Handle untracked files
        untracked_files = subprocess.run(["git", "ls-files", "--others", "--exclude-standard"], capture_output=True, text=True, check=True).stdout.splitlines()
        diff_untracked = ""
        for f in untracked_files:
            if os.path.isfile(f):
                # Using 'git diff --no-index /dev/null <file>' to generate a diff for new files
                res = subprocess.run(["git", "diff", "--no-index", "/dev/null", f], capture_output=True, text=True)
                diff_untracked += res.stdout

        full_diff = diff_committed + diff_unstaged + diff_untracked
        return full_diff if full_diff.strip() else None
    except subprocess.CalledProcessError as e:
        print(f"Error getting git diff: {e.stderr}")
        return None

def call_gemini_api(diff_content):
    """Sends the diff to Gemini API for review."""
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("Error: GEMINI_API_KEY environment variable not set.")
        sys.exit(1)

    prompt = f"""
    You are an expert software engineer and security auditor.
    Review the following code diff for a Pull Request towards the 'main' branch of the Horizon SDV project.
    
    Focus on:
    1.  **Security**: Check for secrets, insecure configurations, or potential vulnerabilities.
    2.  **Best Practices**: Check for code quality, naming conventions, and idiomatic patterns.
    3.  **Correctness**: Identify potential bugs or logical errors.
    4.  **Maintainability**: Suggest improvements for readability and structure.
    5.  **Project Specifics**: Adhere to Horizon SDV standards (e.g., shell scripts should use 'set -e', Terraform should be well-structured).
    
    Code Diff:
    ```diff
    {diff_content}
    ```
    
    Provide your review in a structured format (e.g., markdown) with clear actionable feedback.
    """

    payload = {
        "contents": [
            {
                "parts": [
                    {"text": prompt}
                ]
            }
        ],
        "generationConfig": {
            "temperature": 0.2,
            "topP": 0.8,
            "topK": 40,
            "maxOutputTokens": 2048,
        }
    }

    headers = {"Content-Type": "application/json"}
    response = requests.post(f"{GEMINI_API_URL}?key={api_key}", headers=headers, json=payload)

    if response.status_code == 200:
        return response.json()["candidates"][0]["content"]["parts"][0]["text"]
    else:
        print(f"Error calling Gemini API: {response.status_code} - {response.text}")
        return None

def main():
    base_branch = "main"
    if len(sys.argv) > 1:
        base_branch = sys.argv[1]

    print(f"Comparing current branch with {base_branch}...")
    diff = get_git_diff(base_branch)
    
    if not diff:
        print("No changes found or error occurred.")
        sys.exit(0)

    print("Sending diff to Gemini for agentic review...")
    review = call_gemini_api(diff)
    
    if review:
        print("\n=== AGENTIC PR REVIEW ===\n")
        print(review)
        print("\n==========================\n")
    else:
        print("Failed to get review from Gemini.")
        sys.exit(1)

if __name__ == "__main__":
    main()
