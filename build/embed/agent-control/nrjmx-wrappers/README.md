# nrjmx wrapper scripts for the Agent Control package

Wrapper scripts (`nrjmx` for Linux, `nrjmx.bat` for Windows) that are shipped **only** in the Agent Control (AC)
package, next to `nrjmx.jar` in `infra-agent-ohi-binaries/integrations/`. They are not used by the standalone
release packages or the container image, which install nrjmx through its own package.

## Why they exist

- `nri-jmx` (gojmx) runs `NR_JMX_TOOL` as a single executable, so it cannot be a `java -jar ...` command line.
- The wrappers shipped by the nrjmx package don't work in the AC package: the Linux one looks for a system-wide
  nrjmx installation first, and the Windows one hardcodes its install path. The AC package is a self-contained tree.
- Ours always run the `nrjmx.jar` sitting next to them. Java is taken from `NRIA_JAVA_HOME`, then `JAVA_HOME`,
  then `PATH`.

## Drift check

`upstream-wrapper.sha256` is the sha256 of the upstream Linux wrapper (`/usr/bin/nrjmx` in the nrjmx package).
The `agent_control_artifact.yml` workflow fails if it changes, so that someone reviews whether these scripts need
the same change (new JVM options, classpath, main class...) and then updates the hash.

The workflow also smoke-tests `nrjmx` against the real jar from a path containing spaces.

## Line endings

`.gitattributes` forces LF for `nrjmx` and CRLF for `nrjmx.bat`.
