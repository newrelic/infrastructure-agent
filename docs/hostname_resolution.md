# Hostname resolution in the infrastructure agent

There have been several changes in the way the infrastructure agent handles hostname resolution. The following explains hostname resolution behaviour from v1.0.976 onward.

The agent provides two fields for hostname resolution: `hostname` and `fullHostname`.

### Short hostname (`hostname`)

The name the host knows about itself (referred here as short hostname for disambiguation). It's fetched in Linux via a [`uname` syscall](http://man7.org/linux/man-pages/man2/uname.2.html). If that syscall fails, `/proc/sys/kernel/hostname` is read for this purpose.

### Full hostname (`fullHostname`)

A more accurate version that includes the domain, or how the host is known by the outside world, resolved via DNS lookup on the previously returned value of short-hostname. Full hostname resolution could fail in many cases; a common one is lack of network connectivity.

In case there's problems gathering the full hostname then a fallback procedure tries a command based resolution. This internal resolution launches `hostname -f` command to get a valid result, and if this fails `hostname` command is executed as backup.

## Overriding behaviour

Two flags override both resolutions with the value provided by config:
- `override_hostname`
- `override_hostname_short`

## Version differences

Since version 1.2.6 there is a difference in how the full hostname falls back to the command line based resolution. Versions fallback like this:

- 1.1.9: when full hostname is `localhost`
- 1.2.6: when full hostname is either `localhost` or empty and there was no previous successful full hostname resolved

## Windows

The agent opens the `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters` path under the `LOCAL_MACHINE` registry store. Then, it attempts to read the `Domain` value from that path. If the registry key doesn't exist, the agent quits processing for a hostname.

If the agent gets an empty value from `Domain`, it checks the `DhcpDomain` value from that path. If the registry key doesn't exist, the agent quits processing the hostname. If the agent gets an empty value from `DhcpDomain`, it then checks the `Hostname` value from that path. Again, if the registry key doesn't exist, the agent quits processing the hostname.

Depending on the values that weren't empty in the previous steps, the agent returns a final "hostname" formatted as `hostname.domain` where the first portion is from the `Hostname` registry value and the second portion is from the `Domain` or `DhcpDomain` registry value (whichever isn't empty, `Domain` being preferred). If no `Domain` portion is available, the agent just returns the `Hostname` registry value.
