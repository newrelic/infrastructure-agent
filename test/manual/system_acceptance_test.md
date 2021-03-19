### Why:
As moving forward to support many OS distribution and version we want to unsure agent main functionalities covered and working in each new release.

### How:
Under script folder You will find all commands and assert criteria to run locally this suit case separated by each OS dist package manager.

### Setup:
In order to pass this manual test some requirements are needed:
- Install [newrelic-cli](https://github.com/newrelic/newrelic-cli) with your license key and api key (can be obtained though NR user information)
This tools will allow You to get data from your agent host on NR and validate some use cases.
- Add env var:
    ```shell script
    export NR_LICENSE_KEY=*** NR_API_KEY=*** NR_ACCOUNT_ID=*** DISPLAY_NAME=*** NR_REGION=***
    ```

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
Then no agent is present.

- ***Scenario 10. Package upgrade*** \
Given an old newrelic-infra agent version installed, \
When I upgrade to latest released version, \
Then all host data is sent to NR.
 
- ***Scenario 11. Built in Flex integration is working*** \
Given the latest newrelic-infra agent version installed, \
When agent run nri-flex integration \
Then I flexSample is sent to NR.

- ***Scenario 11. Built in Log-forwarded integration is working*** \
Given the latest newrelic-infra agent version installed, \
When I enable the log-forwarder mode, \
Then my configured logs are sent to NR.

- ***Scenario 12. Built in Prometheus integration is working*** \
Given the latest newrelic-infra agent version installed, \
When I run nri-prometheus integration binary, \
Then the integration returns a message.