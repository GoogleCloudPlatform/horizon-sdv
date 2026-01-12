## Collaboration Rules For external Contributors

1. Contributors may want to raise their ideas in form of Pull Requests

2. External Pull Requests shall be raised against branch “devel” GitHub - GoogleCloudPlatform/horizon-sdv at devel . But shall never be merged to “devel” branch by the community (yet) !

3. Pull Requests raised against branch “main” GitHub - GoogleCloudPlatform/horizon-sdv will be considered as a mistake and shall be rebased onto branch “devel”.

4. Pull Request branch names shall follow a naming convention “contrib/*”

5. Once a contribution request is raised - “contrib/*” branch shall be pulled to our internal repository and attempted to be checked on a selected environment (rebase or cherry-pick may be required to achieve that). IMPORTANT note is that it is currently impossible to pull the contrib branch with its original Author to AGBG repository, so it is required to change the “Author” and “Commiter” to users coming from AGBG GitHub account.

6. New contrib/* branch needs to be approved by Horizon QA&Release Team after trying it first on selected internal environment. It cannot be merged to any of the main environment or integration or devel or main branches, but only left on temporary “try&attempt” branch for testing purposes.

7. Potentially, a contrib branch owner may be asked to update or rebase the contribution due to our internal needs, therefore, updated branch needs to be pulled again.

8. If change is accepted, contrib/* branch can be merged on Google GitHub to the devel branch by approving the Pull Request. IMPORTANT note is that every merge to devel or to main branches on Google GitHub must not be a “Commit Merge” but only “Squashed Merge”.

9. Once the PR is merged to “devel” branch, it is sync’ed again to AGBG GitHub (fast forward merge).

10. Finally, it is merged to environment branch(-es) and then to main branch on AGBG GitHub.