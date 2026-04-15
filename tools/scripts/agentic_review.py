#!/usr/bin/env python3
import os
import subprocess
import sys
import json
import requests
import logging
import time
from urllib3.util import Retry
from requests.adapters import HTTPAdapter

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    stream=sys.stderr
)
logger = logging.getLogger(__name__)

# Constants and Configuration
DEFAULT_MODEL = "gemini-2.5-flash"
MAX_DIFF_SIZE = 100000  # limit diff to ~100k chars to avoid token limits
GEMINI_API_BASE_URL = "https://generativelanguage.googleapis.com/v1beta/models"

def get_git_diff(base_branch="main"):
    """Gets the diff between the current state and the base branch."""
    try:
        # Try to resolve the base branch (it might be a local branch or origin/branch)
        base_ref = None
        for ref in [base_branch, f"origin/{base_branch}"]:
            res = subprocess.run(["git", "rev-parse", "--verify", ref], capture_output=True, text=True)
            if res.returncode == 0:
                base_ref = ref
                break
        
        if not base_ref:
            logger.error(f"Could not resolve base branch '{base_branch}' locally or on origin.")
            return None

        logger.info(f"Using base ref: {base_ref}")
        
        # 1. Get staged/unstaged changes compared to base_ref
        # Use ... to get the diff from the common ancestor (merge base)
        diff_committed = subprocess.run(["git", "diff", f"{base_ref}...HEAD"], capture_output=True, text=True, check=True).stdout
        diff_unstaged = subprocess.run(["git", "diff", "HEAD"], capture_output=True, text=True, check=True).stdout
        
        # 2. Handle untracked files
        untracked_files = subprocess.run(["git", "ls-files", "--others", "--exclude-standard"], capture_output=True, text=True, check=True).stdout.splitlines()
        diff_untracked = ""
        for f in untracked_files:
            if os.path.isfile(f):
                # Only include files smaller than 1MB to avoid bloating the diff
                if os.path.getsize(f) < 1024 * 1024:
                    res = subprocess.run(["git", "diff", "--no-index", "/dev/null", f], capture_output=True, text=True)
                    diff_untracked += res.stdout

        full_diff = diff_committed + diff_unstaged + diff_untracked
        
        if len(full_diff) > MAX_DIFF_SIZE:
            logger.warning(f"Diff size ({len(full_diff)}) exceeds limit ({MAX_DIFF_SIZE}). Truncating...")
            full_diff = full_diff[:MAX_DIFF_SIZE] + "\n\n... [Diff Truncated due to size limit] ..."
            
        return full_diff if full_diff.strip() else None
    except subprocess.CalledProcessError as e:
        stderr_msg = e.stderr.decode() if isinstance(e.stderr, bytes) else e.stderr
        logger.error(f"Error getting git diff: {stderr_msg}")
        return None

def create_requests_session():
    """Creates a requests session with retry logic."""
    session = requests.Session()
    retry = Retry(
        total=3,
        backoff_factor=1,
        status_forcelist=[429, 500, 502, 503, 504],
    )
    adapter = HTTPAdapter(max_retries=retry)
    session.mount("https://", adapter)
    return session

def call_gemini_api(diff_content):
    """Sends the diff to Gemini API for review."""
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        logger.error("GEMINI_API_KEY environment variable not set.")
        sys.exit(1)

    model = os.environ.get("GEMINI_MODEL", DEFAULT_MODEL)
    api_url = f"{GEMINI_API_BASE_URL}/{model}:generateContent?key={api_key}"

    prompt = f"""
    You are an expert software engineer and security auditor.
    Review the following code diff for a Pull Request or Push towards the 'main' branch of the Horizon SDV project.
    
    CRITICAL INSTRUCTIONS:
    1.  **Security**: Check for secrets (keys, tokens), insecure configurations, or potential vulnerabilities.
    2.  **Best Practices**: Check for code quality, naming conventions, and idiomatic patterns.
    3.  **Correctness**: Identify potential bugs, logical errors, or edge cases.
    4.  **Maintainability**: Suggest improvements for readability, structure, and documentation.
    5.  **Project Specifics**: 
        - Shell scripts MUST use 'set -e'.
        - Terraform MUST be modular and follow project structure.
    
    6.  **Actionable Feedback & Fixes**:
        - When suggesting a code change, provide it as a GitHub "Suggested Change" block if possible.
        - Format: 
          ```suggestion
          <your suggested code here>
          ```
        - Be concise and focus on high-impact improvements.
    
    If no issues are found, provide a brief positive summary.
    If issues are found, be specific and provide actionable advice with code suggestions.
    
    Code Diff:
    ```diff
    {diff_content}
    ```
    
    Provide your review in a structured Markdown format with clear headings.
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
            "temperature": 0.1,  # Lower temperature for more deterministic/factual review
            "topP": 0.95,
            "maxOutputTokens": 4096,
        }
    }

    headers = {"Content-Type": "application/json"}
    session = create_requests_session()
    
    try:
        response = session.post(api_url, headers=headers, json=payload, timeout=60)
        response.raise_for_status()
        
        result = response.json()
        if "candidates" in result and result["candidates"]:
            candidate = result["candidates"][0]
            if "content" in candidate and "parts" in candidate["content"]:
                return candidate["content"]["parts"][0]["text"]
            elif "finishReason" in candidate and candidate["finishReason"] != "STOP":
                logger.warning(f"Gemini finished with reason: {candidate['finishReason']}")
                return f"Review partial or blocked. Reason: {candidate['finishReason']}"
        
        logger.error(f"Unexpected API response structure: {json.dumps(result)}")
        return None
        
    except requests.exceptions.RequestException as e:
        logger.error(f"API Request failed: {e}")
        return None

def main():
    base_branch = "main"
    if len(sys.argv) > 1:
        base_branch = sys.argv[1]

    logger.info(f"Comparing current branch with {base_branch}...")
    diff = get_git_diff(base_branch)
    
    if not diff:
        print("## Agentic Review\nNo changes found or error occurred during diff generation.")
        return

    logger.info("Sending diff to Gemini for agentic review...")
    review = call_gemini_api(diff)
    
    if review:
        print("\n## Agentic PR Review 🤖\n")
        print(review)
        print("\n---\n*Review powered by Gemini Flash 2.0*")
    else:
        print("## Agentic Review\nFailed to get review from Gemini. Check workflow logs for details.")
        sys.exit(1)

if __name__ == "__main__":
    main()
