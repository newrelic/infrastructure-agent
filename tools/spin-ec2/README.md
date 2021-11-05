# AWS tools for infrastructure-agent
spin-ec2 tools is used for provisioning machines in AWS

## Examples
The following example shows how to deploy infrastructure agent canaries:

```bash
    go build .
    # Display help page
    ./spin-ec2 canaries -h

    # Export AWS configuration
    export AWS_PROFILE=<profile>
    export export AWS_REGION=<region>

    # Provision canaries (ansible password is necessary for windows hosts)
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -x <ansible_password> 
    
    # Provision canaries by platform ( linux | windows | macos )
    # windows canaries
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -p windows -x ansible_password 
    # linux canaries
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -p linux

    # Cleanup old machines. This command will terminate all the instances except the ones
    # that have the latest 2 versions of infra-agent installed.
    ./spin-ec2 canaries prune
```