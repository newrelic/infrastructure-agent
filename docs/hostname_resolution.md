# Hostname resolution in the infra agent

There has been several changes in the hostname resolution. This explains only behaviour since `v1.0.976` upwards.

Agent provides 2 fields: hostname and fullHostname
- hostname: is the name that host knows about itself (referred here as short-hostname for disambiguation) 
- fullHostname: tries to be more accurate and include domain, or how host is known from the outside

## Short hostname

It's fetched in Linux via `uname` syscall: http://man7.org/linux/man-pages/man2/uname.2.html
If that syscall fails then `/proc/sys/kernel/hostname` is read for this purpose.

## Full hostname

This one is resolved via DNS lookup on the previously returned value of short-hostname.
Full-hostname resolution could fail on many cases, a common one is lack of network connectivity.
In case there's problems gathering full-hostname then a fallback procedure tries a command based resolution.
This internal resolution launches `hostname -f` command to get a valid result, and if this fails then `hostname` command is executed as backup.

## Overriding behaviour

There are 2 flags that override both resolutions with the value provided by config:
- override_hostname
- override_hostname_short

## Version differences

Since version `v1.2.6` there is a difference is in how full-hostname falls back to the command based resolution. Versions fallback this way:

- 1.1.9: when full-hostname is localhost
- 1.2.6: when full-hostname is either localhost or empty and there was no previous successful full-hostname resolved before

## Windows

- The agent opens the `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters` path under the 
`LOCAL_MACHINE` registry store.
- Then attempts to read the `Domain` value from that path. If the registry key doesn't exist, the 
agent quits processing for a hostname.
- If the agent gets an empty value from `Domain`, it will then check the `DhcpDomain` value from 
that path. If the registry key doesn't exist, the agent quits processing for a hostname.
- If the agent gets an empty value from `DhcpDomain`, it will then check the `Hostname` value from 
that path. If the registry key doesn't exist, the agent quits processing for a hostname.
- Depending on the values that weren't empty from the prior steps, the agent will return a final 
"hostname" formatted as `hostname.domain` where the first portion is from the `Hostname` registry 
value and the second portion is from the `Domain` or `DhcpDomain` registry value (whichever was not 
empty, preferring `Domain`).
- If no `Domain` portion was available, the agent just returns the `Hostname` registry value.
