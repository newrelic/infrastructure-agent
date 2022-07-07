## PGP Signed tarball packages
Since version 1.27.3 the tarball packages are signed using [GPG](https://gnupg.org/). 

This means that within the package `newrelic-infra_linux_1.27.3_arm64.tar.gz` we will also provide a file with 
the signature `newrelic-infra_linux_1.27.3_arm64.tar.gz.asc` that will allow verifying the integrity of the package.

## How packages are signed
Packages are signed using [GPG](https://gnupg.org/) public/private key pairs. New Relic will sign the packages using its 
`private key` and users can verify the packages integrity using the `public key`.

```shell
gpg --sign --armor --detach-sig $targz_file
```

## Verify downloaded package integrity
To verify the **authenticity** we need to ensure that we only download the public key from a trustable source using
the secure protocol HTTPS. Once we have the public key we can use `gpg` program to verify the package **integrity**.

#### Download the public key using HTTPS from a trustable source.
```shell
$ curl -o newrelic-infra.gpg https://download.newrelic.com/infrastructure_agent/gpg/newrelic-infra.gpg
```

#### Import the public key into the system
```shell
$ gpg --import newrelic-infra.gpg

gpg: key BB29EE038ECCE87C: public key "infrastructure-eng <infrastructure-eng@newrelic.com>" imported
gpg: Total number processed: 1
gpg:               imported: 1

```

#### Get public key's **fingerprint**
```shell
$ gpg --show-keys --fingerprint newrelic-infra.gpg

pub   rsa4096 2016-10-26 [SCEA]
      A758 B3FB CD43 BE8D 123A  3476 BB29 EE03 8ECC E87C
uid                      infrastructure-eng <infrastructure-eng@newrelic.com>
```

In the example above, the public key is `A758 B3FB CD43 BE8D 123A  3476 BB29 EE03 8ECC E87C`

#### Mark the downloaded key as trustable
After downloading the key from a trustable source you can mark the key as trustable to avoid warning messages. 

Run the following command to edit they key using `gpg>` prompt and enter:
* `trust`
* `5` for ultimately trust
* `y` to confirm
* `quit`

```shell
$ gpg --edit-key "A758 B3FB CD43 BE8D 123A  3476 BB29 EE03 8ECC E87C" 

pub  rsa4096/BB29EE038ECCE87C
     created: 2016-10-26  expires: never       usage: SCEA
     trust: unknown       validity: unknown
[ unknown] (1). infrastructure-eng <infrastructure-eng@newrelic.com>

gpg> trust
pub  rsa4096/BB29EE038ECCE87C
     created: 2016-10-26  expires: never       usage: SCEA
     trust: unknown       validity: unknown
[ unknown] (1). infrastructure-eng <infrastructure-eng@newrelic.com>

Please decide how far you trust this user to correctly verify other users' keys
(by looking at passports, checking fingerprints from different sources, etc.)

  1 = I don't know or won't say
  2 = I do NOT trust
  3 = I trust marginally
  4 = I trust fully
  5 = I trust ultimately
  m = back to the main menu

Your decision? 5
Do you really want to set this key to ultimate trust? (y/N) y

pub  rsa4096/BB29EE038ECCE87C
     created: 2016-10-26  expires: never       usage: SCEA
     trust: ultimate      validity: unknown
[ unknown] (1). infrastructure-eng <infrastructure-eng@newrelic.com>
Please note that the shown key validity is not necessarily correct
unless you restart the program.

gpg> quit
```

## Verify the integrity of the package

After importing New Relic public key into the system we can use `gpg` to verify the integrity of a signed package. The 
command will output `Good signature from` message and it will return `0` exit code when the package is signed correcty. 

```shell
$ gpg --verify newrelic-infra_linux_1.27.3_arm64.tar.gz.asc newrelic-infra_linux_1.27.3_arm64.tar.gz

gpg: Signature made Thu Jun  9 09:52:38 2022 UTC
gpg:                using RSA key A758B3FBCD43BE8D123A3476BB29EE038ECCE87C
gpg: checking the trustdb
gpg: marginals needed: 3  completes needed: 1  trust model: pgp
gpg: depth: 0  valid:   1  signed:   0  trust: 0-, 0q, 0n, 0m, 0f, 1u
gpg: Good signature from "infrastructure-eng <infrastructure-eng@newrelic.com>" [ultimate]

echo $?
0

```

When a package is not signed correctly or is corrupted the exit code will be `1` and the message output will containe
`Bad signature from`
```shell
$ gpg --verify corrupted_package.tar.gz.asc corrupted_package.tar.gz

gpg: Signature made Thu Jun  9 09:52:38 2022 UTC
gpg:                using RSA key A758B3FBCD43BE8D123A3476BB29EE038ECCE87C
gpg: BAD signature from "infrastructure-eng <infrastructure-eng@newrelic.com>" [ultimate]

echo $?
1
```
