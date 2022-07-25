# New Relic Signing Key
This project consists on building a debian package, `newrelic-signing-keys`, which contains the public key for both the 
key that is currently be used for signing (“current key”), and the key that will be used after the rotation takes place 
(“future key”). The future key is generated in advance, but not used to sign packages until a reasonable time 
(“update window”) passes.

`newrelic-signing-keys` would be a dependency of all New Relic core packages. As users upgrade packages, they will get the 
latest version of newrelic-signing-keys, which adds to their system’s truststore the future key. After the update window 
passes, New Relic will start to sign packages and repository metadata with the future key, effectively making it the 
current key. The previous current key ("old key") is removed from the newrelic-signing-keys package, and a new future 
key is generated and added to the package.

The following diagram illustrates the process. In State 1, 0xAAAA is the current key, and 0xBBBB the future key. All NR 
packages are signed with 0xAAAA, but the newrelic-signing-keys package is already deploying 0xBBBB as a valid key. After 
some time passes, State 2 is reached when 0xBBBB becomes the current key and packages start to get signed with it. A new
key, 0xCCCC is generated and its public counterpart deployed, and the process restarts with 0xBBBB being the current key 
and 0xCCCC the future key.

![Signing Key Diagram](doc/signing_key_diagram.png "Signing Key Diagram")

## Requirements
* Docker

## Build

Current command will leave the generated deb package under `./pkg` folder
```shell
PGK_VERSION=1.2.3 make build
```