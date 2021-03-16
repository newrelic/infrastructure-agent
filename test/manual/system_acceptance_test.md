### Test cases:
As moving forward to support many OS distribution and version we want to unsure agent main functionalities covered and working in each new release.  

### Acceptance and Criteria:
- ***Scenario 1. Package naming follow dist convention*** \
Given a new newrelic-infra agent version to install, \
When inspect package name, \
Then name match with the previous version. 

- ***Scenario 2. Package contains all required files*** \
Given the latest newrelic-infra agent version installed, \
When I list all files installed to my system, \
Then all match with previous version. 

- ***Scenario 3. Check agent version*** \
Given the latest newrelic-infra agent version installed, \
When I review the version number, \
Then it matches with latest released version.   

- ***Scenario 4. Service is working***
Given the latest newrelic-infra agent version installed, \    
When I run the agent, \
Then all host data is sent to NR.
    
- ***Scenario 5. Package metadata is valid*** \
Given the latest newrelic-infra agent version installed, \
When I inspect package metadata, \
Then all Company/team/version is present.

- ***Scenario 6. Package signature is valid*** \
Given the latest newrelic-infra agent version installed, \
When I inspect package signature, \
Then it matches with previous released version. 

- ***Scenario 7. Agent privileged mode is working*** \
Given the latest newrelic-infra agent version installed as privileged mode, \
When I run the agent, \
Then all host data is sent to NR.

- ***Scenario 8. Agent unprivileged mode is working*** \
Given the latest newrelic-infra agent version installed as unprivileged mode, \
When I run the agent, \
Then all host data is sent to NR.

- ***Scenario 9. Package uninstall*** \
Given the latest newrelic-infra agent version installed as unprivileged mode, \
When I uninstall the newrelic-infra agent package, \
???

- ***Scenario 10. Package upgrade*** \
Given an old newrelic-infra agent version installed, \
When I upgrade to latest released version, \
Then all host data is sent to NR.
 
- ***Scenario 11. Built in Flex integration is working*** \
Given the latest newrelic-infra agent version installed, \
???

- ***Scenario 11. Built in Log-forwarded integration is working*** \
Given the latest newrelic-infra agent version installed, \
???

- ***Scenario 11. Built in Prometheus integration is working*** \
Given the latest newrelic-infra agent version installed, \
???
 