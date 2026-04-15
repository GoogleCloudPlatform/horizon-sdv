#!/usr/bin/env python3
import os
import subprocess
import sys

# This script performs a basic OSS dependency scan.
# It looks for common dependency files and lists them.
# In a real-world scenario, this could be extended to use tools like 'npm audit' or 'pip-audit'.

def scan_python_dependencies():
    findings = []
    for root, dirs, files in os.walk("."):
        if "requirements.txt" in files:
            file_path = os.path.join(root, "requirements.txt")
            findings.append(f"### Python: {file_path}")
            with open(file_path, "r") as f:
                content = f.read().strip()
                findings.append(f"```\n{content}\n```")
    return findings

def scan_node_dependencies():
    findings = []
    for root, dirs, files in os.walk("."):
        if "package.json" in files:
            file_path = os.path.join(root, "package.json")
            findings.append(f"### Node.js: {file_path}")
            # Try to run npm audit if possible, else just list the file
            findings.append(f"Dependency file found at `{file_path}`")
    return findings

def main():
    print("## OSS Scan List and Dependency Audit")
    
    python_findings = scan_python_dependencies()
    node_findings = scan_node_dependencies()
    
    if not python_findings and not node_findings:
        print("No open-source dependency files found.")
        return

    if python_findings:
        print("\n## Python Dependencies")
        for finding in python_findings:
            print(finding)

    if node_findings:
        print("\n## Node.js Dependencies")
        for finding in node_findings:
            print(finding)

    print("\n---")
    print("*Note: This is a preliminary scan. For a full security audit, please run dedicated security tools like Snyk or Trivy.*")

if __name__ == "__main__":
    main()
