#!/usr/bin/env python3
import os
import sys
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(levelname)s: %(message)s',
    stream=sys.stderr
)
logger = logging.getLogger(__name__)

# Configurable constants
IGNORE_DIRS = {".git", "__pycache__", "node_modules", ".terraform", "venv", ".venv"}
DEPENDENCY_FILES = {
    "python": ["requirements.txt", "Pipfile", "pyproject.toml"],
    "nodejs": ["package.json", "package-lock.json", "yarn.lock"],
    "go": ["go.mod", "go.sum"],
}

def scan_dependencies():
    """Scans the repository for common dependency files."""
    findings = {lang: [] for lang in DEPENDENCY_FILES}
    
    for root, dirs, files in os.walk("."):
        # Prune ignored directories
        dirs[:] = [d for d in dirs if d not in IGNORE_DIRS]
        
        for lang, target_files in DEPENDENCY_FILES.items():
            for f in files:
                if f in target_files:
                    full_path = os.path.join(root, f)
                    findings[lang].append(full_path)
    
    return findings

def generate_report(findings):
    """Generates a markdown report of found dependencies."""
    if not any(findings.values()):
        return "## OSS Dependency Scan\nNo common open-source dependency files were found."

    report = ["## OSS Dependency Scan 🔍"]
    report.append("The following dependency files were identified in this PR/repository:\n")

    for lang, files in findings.items():
        if files:
            report.append(f"### {lang.capitalize()}")
            for f in sorted(files):
                report.append(f"- `{f}`")
            report.append("")

    report.append("---")
    report.append("### Recommendations")
    report.append("1. **Vulnerability Scanning**: Ensure these files are scanned by tools like `Snyk`, `Trivy`, or `GitHub Dependency Graph`.")
    report.append("2. **Updates**: Regularly check for outdated dependencies using `npm outdated` or `pip list --outdated`.")
    report.append("3. **License Compliance**: Verify that all third-party libraries adhere to the project's license policy.")
    report.append("\n*Note: This is a preliminary discovery scan.*")

    return "\n".join(report)

def main():
    try:
        findings = scan_dependencies()
        report = generate_report(findings)
        print(report)
    except Exception as e:
        logger.error(f"Error during OSS scan: {e}")
        print("## OSS Dependency Scan\nError: Failed to complete the scan.")
        sys.exit(1)

if __name__ == "__main__":
    main()
