# Git Repo Configuration

## Table of Contents
- [Repository Settings](#repository-settings)
  - [Teams](#teams)
  - [Roles](#roles)
  - [Settings](#settings)
- [Branch Rulesets](#branch-rulesets)
  - [All branches](#all-branches)
    - [Allowed Branch Names](#allowed-branch-names)
  - [main branch](#main-branch)
  - [release branches](#release-branches)
- [Tag Rulesets](#tag-rulesets)
  - [Allowed Tag Names](#allowed-tag-names)
- [PR Template](#pr-template)

---
> **IMPORTANT:** 
> Please ensure that when copying regex values that the resulting regex looks exactly as it does in the published document - i.e. any escaped characters are not present (e.g. | instead of \\| or $ instead of \\$ )

# Repository Settings

## Teams
Create a `ReleaseManagers` team and a `CodeOwners` team and add required members to each team.

## Roles

| Developers | Release Managers | Code Owners |
| --- | --- | --- |
| • Write access to repository<br>• Create feature/fix branches<br>• Raise and review PRs<br>• Cannot bypass repository rules | • Write access to repository<br>• Members of GitHub team `ReleaseManagers`<br>• Responsible for:<ul><li>Approving Pull Requests<li>Creating and managing `rel/*` branches<li>Release tagging</ul>• May be configured with ruleset bypass permission | • Write access to repository<br>• Members of GitHub team `CodeOwners`<br>• Responsible for:<ul><li>Approving Pull Requests</ul>• May be configured with ruleset bypass permission |

## Settings
To set the following, click on repository name in GitHub, then **Settings** tab.

| Item | Setting | Explanation |
| --- | --- | --- |
| Default branch | `main` | |
| Features / Pull Requests | on | |
| Features / Pull Requests / Creation allowed by: | Collaborators only | Collaborators are those who have repository write access. |
| Pull requests / Allow merge commits | off | |
| Pull requests / Allow squash merging | on | This is the default mechanism for merging approved PRs |
| Pull requests / Allow squash merging / Default commit message | Pull request title and description | |
| Pull requests / Allow rebase merging | on | Only used in exceptional circumstances when multiple commits from a single feature/fix branch are desired (and are already squashed appropriately). |
| Pull requests / Always suggest updating pull request branches | on | Provides a prompt when a feature / fix branch is behind `main` - useful to remind developers to perform a rebase. |
| Pull requests / Allow auto-merge | off | |
| Pull requests / Automatically delete head branches | on | |

---

# Branch Rulesets

## All branches

| Item | Setting | Comment |
| --- | --- | --- |
| Enforcement status | Active | |
| Bypass list | empty | |
| Target branches | All branches | |
| Require signed commits | on | |
| Restrict branch names / Add restriction / Applies to | Branch name | |
| Restrict branch names / Add restriction / Requirement | Must match a given regex pattern | |
| Restrict branch names / Add restriction / Matching pattern | `^(main\|feature/[^/]+\|fix/[^/]+\|contrib/feature/[^/]+\|contrib/fix/[^/]+\|rel/\d+\.\d+\.\d+)\$` | A combined regex is required to ensure that GitHub doesn’t require a branch name to match all regexes simultaneously. The regex listed should match any of the Allowed Branch Names in the table below.|
| Restrict branch names / Add restriction / Description | Branch name must match one of the following: `^main$` , `^feature/[^/]+$` , `^fix/[^/]+$` , `^contrib/feature/[^/]+$` , `^contrib/fix/[^/]+$` , `^rel/\d+\.\d+\.\d+$` | |

### Allowed Branch Names

| Branch Type | Regex |
| --- | --- |
| Main | `^main$` |
| Feature | `^feature/[^/]+$` |
| Fix | `^fix/[^/]+$` |
| External feature | `^contrib/feature/[^/]+$` |
| External fix | `^contrib/fix/[^/]+$` |
| Release | `^rel/\d+\.\d+\.\d+$` |

## main branch
`main` is the default branch in the repo; it is the only permanent branch.  
The following ruleset should be created for the `main` branch.

| Item | Setting | Comment |
| --- | --- | --- |
| Enforcement status | Active | |
| Bypass list | none | |
| Target branches / Include by pattern | `main` | |
| Restrict deletions | on | |
| Require linear history | on | Prevents merge commits |
| Require a pull request before merging | on | |
| Require a PR / Required approvals | 3 | |
| Require a PR / Dismiss stale pull request approvals when new commits are pushed | on | |
| Require a PR / Require review from specific teams / Reviewer | `ReleaseManagers` (Approvals: 1, File patterns: *)<br>`CodeOwners` (Approvals: 1, File patterns: *) | |
| Require a PR / Restrict who can dismiss pull request reviews | Repository admin | |
| Require a PR / Require approval of the most recent reviewable push | on | |
| Allowed merge methods | Squash, Rebase | |
| Block force pushes | on | |

## release branches
The following ruleset should be created for release branches.

| Item | Setting | Comment |
| --- | --- | --- |
| Enforcement status | Active | |
| Bypass list | `ReleaseManagers`, `CodeOwners` | |
| Target branches / Include by pattern | `rel/*.*.*` | Specifies that this ruleset applies only to branches matching this format. Any matching branches must additionally match the regex set in the **Set branch names** restriction. |
| Restrict Creations | on | Only users with bypass permissions can create matching branches |
| Restrict updates | on | Only users with bypass permissions can push or merge updates |
| Restrict deletions | on | |
| Require linear history | on | Prevents merge commits |
| Require signed commits | on | |
| Block force pushes | on | |
| Restrict branch names | on | |
| Restrict branch names / Add restriction / Applies to | Branch name | |
| Restrict branch names / Add restriction / Requirement | Must match a given regex pattern | |
| Restrict branch names / Add restriction / Matching pattern | `^rel/\d+\.\d+\.\d+$` | Limits creation of branches matching the "include pattern" to names matching this regex. |
| Restrict branch names / Add restriction / Description | Branch name must match this pattern (e.g., `rel/1.0.0`, `rel/24.3.16`, or `rel/12.10.135`) | |

---

# Tag Rulesets

| Item | Setting | Comment |
| --- | --- | --- |
| Enforcement status | Active | |
| Bypass list | `ReleaseManagers`, `CodeOwners` | |
| Target tags | All tags | |
| Restrict creations | on | |
| Restrict updates | on | |
| Restrict deletions | on | |
| Require signed commits | on | |
| Restrict tag names / Add restriction / Applies to | Tag name | |
| Restrict tag names / Add restriction / Requirement | Must match a given regex pattern | |
| Restrict tag names / Add restriction / Matching pattern | `^(R\d+\.\d+\.\d+\|Rel\d+\.\d+\.\d+-RC\d+)$` | A combined regex is required to ensure that GitHub doesn’t require a tag name to match all regexes simultaneously. |
| Restrict tag names / Add restriction / Description | Tag name must match one of: `^R\d+\.\d+\.\d+$` , `^Rel\d+\.\d+\.\d+-RC\d+$` (i.e., `R<X>.<Y>.<Z>` or `Rel<X>.<Y>.<Z>-RC<A>` where X, Y, Z, A are numbers) | |

### Allowed Tag Names

| Tag Type | Pattern | Example | Regex |
| --- | --- | --- | --- |
| Release | `R<X>.<Y>.<Z>` | `R1.0.0` | `^R\d+\.\d+\.\d+$` |
| Release Candidate | `Rel<X>.<Y>.<Z>-RC<A>` | `Rel1.0.0-RC1` | `^Rel\d+\.\d+\.\d+-RC\d+$` |

---

# PR Template
The following template will be set for the Description field of all Pull Requests.

To set up, create the following file at the root of the repo at `.github/pull_request_template.md` and push to `main`. All new PRs targeting `main` will automatically use this template.


```markdown
> [!IMPORTANT]
> Delete the section(s) that do not apply before submitting this PR.

---

# Feature / Fix PR
> Delete this section if this is a Release PR.

## Purpose of Feature/Fix
<!-- Why is this change required? -->

## Concise Description
<!-- Brief description of the implementation. -->

## Dependencies
<!-- List any hard or soft dependencies. Enter N/A if none. -->

## Open Source Component Changes
<!-- List any OSS components added, updated or modified, including justification. Enter N/A if none. -->

## Suggested Release Note
<!-- Suggested wording for inclusion in release notes. -->

## Testing
<!-- Move this section to a PR comment -->
<!-- Describe what was tested, outcomes and evidence. -->

---

# Release PR
> Delete this section if this is a Feature/Fix PR.

## Relevant Release Details
<!-- Include Release Version, Release Date -->

## Features/Fixes Included in this Release
<!-- Brief synopsis of each feature/fix included in the release. -->
<!-- Note any minor fixes made directly on Release branch. -->

<!-- List any superficial edits, documentation updates or minor fixes made directly on the release branch. Enter N/A if none. -->

---

# General Checklist

- [ ] Branch is current with `main`
- [ ] All commits are signed
- [ ] Relevant testing has been completed
- [ ] Documentation has been updated where required
- [ ] PR description has been completed and reviewed