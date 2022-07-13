# AWS tools for infrastructure-agent
spin-ec2 tools is used for provisioning machines in AWS

## Examples
The following example shows how to deploy infrastructure agent canaries:

```shell
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
    # macos canaries
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -p macos
    
    # use custom prefix instead of canary
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -f prefix
      
    # use custom repository
    ./spin-ec2 canaries provision -v v<agent_version> -l <license_key> -r repository_url 

    # Cleanup old machines. This command will terminate all the instances except the ones
    # that have the latest 2 versions of infra-agent installed.
    ./spin-ec2 canaries prune
```

## Examples with Make
```shell

# use custom prefix instead of canary
ANSIBLE_PASSWORD_WINDOWS='PASS' NR_LICENSE_KEY='LICENSE' VERSION=1.24.4 PREFIX='my-own-prefix' make canaries

# use custom repository for packages 
ANSIBLE_PASSWORD_WINDOWS='PASS' NR_LICENSE_KEY='LICENSE' VERSION=1.24.4 REPO='http://nr-downloads-ohai-testing.s3-website-us-east-1.amazonaws.com/infrastructure_agent' make canaries

```
