# nrjmx wrapper scripts for the Agent Control package

Wrapper scripts (`nrjmx` for Linux, `nrjmx.bat` for Windows) that are shipped **only** in the Agent Control (AC)
package, next to `nrjmx.jar` in `infra-agent-ohi-binaries/integrations/`. They are not used by the standalone
release packages or the container image, which install nrjmx through its own package.

## Why they exist

- gojmx, the client library used by `nri-jmx` and other JMX-based integrations, runs nrjmx as a single executable
  (`<executable> -v2`), so it cannot be a `java -jar ...` command line.
- The wrappers shipped by the nrjmx package don't work in the AC package: the Linux one looks for a system-wide
  nrjmx installation first, and the Windows one hardcodes its install path. The AC package is a self-contained tree.
- Ours always run the `nrjmx.jar` sitting next to them. Java is taken from `NRIA_JAVA_HOME`, then `JAVA_HOME`,
  then `PATH`.

## How the integrations find them

gojmx looks for the nrjmx executable in this order: the `NR_JMX_TOOL` environment variable, a file named `nrjmx`
(`nrjmx.bat` on Windows) next to the running integration, and finally the OS default (`/usr/bin/nrjmx` on Linux),
which does not exist in the AC package. The sibling lookup comes from newrelic/nrjmx#221, so the integrations must be
built with a gojmx release that includes it. Because the scripts are copied next to the integrations, no environment
variable is needed.

## Drift check

`upstream-wrapper.sha256` is the sha256 of the upstream Linux wrapper (`/usr/bin/nrjmx` in the nrjmx package).
The `agent_control_artifact.yml` workflow fails if it changes, so that someone reviews whether these scripts need
the same change (new JVM options, classpath, main class...) and then updates the hash.

The workflow also smoke-tests `nrjmx` against the real jar from a path containing spaces.

## Line endings

`.gitattributes` forces LF for `nrjmx` and CRLF for `nrjmx.bat`.
