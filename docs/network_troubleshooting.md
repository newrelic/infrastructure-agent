# Troubleshooting New Relic Infrastructure Agent Networking Issue

## Check networking reachability without the infrastructure agent

Verify that the New Relic’s infrastructure endpoint is reachable from the host where the infrastructure agent is installed. If the following commands show that the endpoints are not reachable, the issue is not related with the infrastructure-agent, instead, with the user's network. In order to use New Relic’s products, the following endpoint should be reachable from the host’s network:

**Windows**

```powershell
$ Invoke-WebRequest -Uri "https://infra-api.newrelic.com/cdn-cgi/trace"
```

**EXPECTED OUTPUT**

```
StatusCode        : 200
StatusDescription : OK
Content           : fl=367f145
                    h=infra-api.newrelic.com
                    ip=XX.YY.ZZ.ZZ
                    ts=1696347189.607
                    visit_scheme=https
                    uag=Mozilla/5.0 (Windows NT; Windows NT 10.0; en-US) WindowsPowerShell/5.1.22621.963
                    colo=MAD
                    sliver=none
                    htt...
RawContent        : HTTP/1.1 200 OK
                    Transfer-Encoding: chunked
                    Connection: keep-alive
                    Access-Control-Allow-Origin: *
                    CF-RAY: 81063def0905384e-MAD
                    X-Frame-Options: DENY
                    X-Content-Type-Options: nosniff
                    Cache-Control...
Forms             : {}
Headers           : {[Transfer-Encoding, chunked], [Connection, keep-alive], [Access-Control-Allow-Origin, *], [CF-RAY, 81063def0905384e-MAD]...}
Images            : {}
InputFields       : {}
Links             : {}
ParsedHtml        : System.__ComObject
RawContentLength  : 284
```

**Linux**

1. **Resolve the domain:** Use nslookup or dig to validate that the Linux system can resolve the domain to an IP address:

```
$ nslookup infra-api.newrelic.com
```

The output should contain a name and IP address.

2. **Ping the server:** Try pinging the IP address obtained above to ensure you can reach the server.

```
$ ping 162.247.241.2
```

The server should be reachable.

3. **Traceroute:** Use traceroute to check the network path between your system and the server:

```
$ traceroute -I infra-api.newrelic.com
```

Analyze the output to identify potential network issues, such as high latency or packet loss.

4. **Curl:** Use curl to check if the endpoint is reachable:

```bash
$ curl -v https://infra-api.newrelic.com/cdn-cgi/trace

fl=366f101
h=infra-api.newrelic.com
ip=XX.YY.ZZ.ZZ
ts=1696347098.76
visit_scheme=https
uag=curl/7.88.1
colo=MAD
sliver=none
http=http/1.1
loc=ES
tls=TLSv1.3
sni=plaintext
warp=off
gateway=off
rbi=off
kex=X25519
* Connection #0 to host infra-api.newrelic.com left intact
```

## Run NRDIAG with endpoint’s connection tests

[Code Implementation](https://github.com/newrelic/newrelic-diagnostics-cli/blob/main/tasks/infra/agent/connect.go#L48)

```bash
./nrdiag -t Infra/Agent/Connect
```

**Expected output:**

```
Check Results
-------------------------------------------------
Info     Base/Env/CollectEnvVars [Gathered Environment variables of current shell.]
Success  Base/Config/Collect
Success  Base/Config/Validate
Success  Base/Config/LicenseKey
Success  Base/Config/ValidateLicenseKey
Success  Infra/Config/Agent
Success  Infra/Agent/Connect
3 results not shown: 3 None
See nrdiag-output.json for full results.
```

**Output when endpoints are not reachable:**

```
Failure - Infra/Agent/Connect

There was an error connecting to https://log-api.newrelic.com
Please check network and proxy settings and try again or see -help for more options.
Error = Get "https://log-api.newrelic.com": dial tcp: lookup log-api.newrelic.com on [::1]:53: read udp [::1]:0->[::1]:53: i/o timeout
See https://docs.newrelic.com/docs/new-relic-solutions/get-started/networks/#infrastructure for more information.
```

Note that the nrdiag Connect task result is shown in stdout, it should be collected with the `nrdiag_output.zip` file.

## Retrieve networking logs from the infrastructure agent

This configuration will enable a Golang’s HTTP tracer that will log additional information for each networking request done by the infrastructure agent. More information: [HttpTracer](https://pkg.go.dev/net/http/httptrace).


**Sample output for each HTTP request:**

```
time="2023-10-04T17:08:10+02:00" level=debug action=GetConn component=HttpTracer hostPort="staging-infra-api.newrelic.com:443" requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=DNSStart component=HttpTracer requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=DNSDone component=HttpTracer duration=28ms requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=ConnectStart addr="162.247.241.2:443" component=HttpTracer network=tcp requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=ConnectDone addr="162.247.241.2:443" component=HttpTracer duration=17ms error="<nil>" network=tcp requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=TLSHandshakeStart component=HttpTracer requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=TLSHandshakeDone component=HttpTracer duration=82ms requester=testing version=1.3
time="2023-10-04T17:08:10+02:00" level=debug action=GotConn component=HttpTracer duration=130ms idleTime=0s requester=testing wasIdle=false
time="2023-10-04T17:08:10+02:00" level=debug action=WroteHeaders component=HttpTracer requester=testing
time="2023-10-04T17:08:10+02:00" level=debug action=WroteRequest component=HttpTracer error="<nil>" requester=testing
time="2023-10-04T17:08:11+02:00" level=debug action=GotFirstResponseByte component=HttpTracer duration=1208ms requester=testing
```

Each log line represents a different part of the network request process:

1. GetConn → create a new request

2. DNSStart & DNSDone → backend IP was resolved

3. ConnectStart & ConnectDone

4. TLSHandshakeStart & TLSHandshakeDone

5. GotConn

6. WroteHeaders & WroteRequest  → Request written into backend

7. GotFirstResponseByte → Response from backend

## Configuration

1. Enable trace logs with the HttpTracer feature
2. Start the infrastructure agent for a few minutes (~5min)
3. Gather the generated logs

**On-host**

```
log:
  level: debug
  include_filters:
    component:
      - HttpTracer
```

**K8S**

Modify the “nri-bundle” or kubelet daemonset to include the following documentation:

```
env: 
  - name: "NRIA_LOG_INCLUDE_FILTERS"
    value: |
      component:
        - "HttpTracer"
  - name: "NRIA_LOG_LEVEL"
    value: "trace"
```

### Common errors shown in logs:

#### x509 certificate signed by unknown authority

```
time="2023-08-31T10:47:09Z" level=warning msg="Collector endpoint not reachable, retrying..." collector_url="https://staging-infra-api.newrelic.com" component=AgentService error="Head \"https://staging-infra-api.newrelic.com\": tls: failed to verify certificate: x509: certificate signed by unknown authority" service=newrelic-infra
```

Certificates to New Relic’s endpoint are not embedded with the infrastructure agent, instead they are retrieved from the host. Golang looks at the following paths to verify HTTPS requests:

To fix the issue, Let’s Encrypt root authority certificates must be installed on the host, they are normally provided by the “ca-certificates” Linux package.

#### Proxy being used without proper configuration

```
time="2023-08-25T12:00:58+01:00" level=error msg="metric sender can't process" component=MetricsIngestSender error="error sending events: Post \"https://infra-api.newrelic.com/infra/v2/metrics/events/bulk\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)" postCount=19310 sendErrorCount=1
```

1. Verify all the Proxy configuration values are provided to the infra-agent. Filter the agent logs to find the used configuration:

```
$ cat newrelic-infra.logs | grep "Loaded configuration."
```

Assert the corresponding [values](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/configuration/infrastructure-agent-configuration-settings/#proxy-variables).

2. Use `curl` to check if the endpoint is reachable via the proxy:

```
$ curl --proxy http://YOUR_PROXY:80 https://infra-api.newrelic.com/
```

#### DNS issues

If the agent logs contain the following error, it might be related to a system’s DNS issue.

```
context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

To easily troubleshoot DNS requests to the ingest endpoint, if the infra agent is configured with the HttpTracer component, enabled, it runs 5 different checks on startup to help narrow down the issue:

1. Configured Agent’s HTTP client: tests the ingest endpoint reachability by using the HTTP client that the Agent uses for all of its requests (configured with user’s configuration, e.g. proxy settings)
2. Plain HTTP client: no configuration applied to the client, just the plain HTTP client provided by Golang.
3. Plain HTTP	client with HEAD request.
4. Custom Golang resolver: Instead of using the system’s DNS resolver, uses one implemented by the Go standard library.
5. Public DNS server: It modifies the HTTP client to use Cloudflare’s public DNS (1.1.1.1) to test the endpoint’s reachability.

**Sample output:**

```
time="2023-10-04T17:04:15+02:00" level=info msg="Checking network connectivity..."
time="2023-10-04T17:04:15+02:00" level=info msg=" ====== CHECKING ENDPOINT REACHABILITY USING CONFIGURED AGENT'S HTTP CLIENT ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:16+02:00" level=info msg=" ====== ENDPOINT REACHABILITY USING CONFIGURED AGENT'S HTTP CLIENT SUCCEED ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:16+02:00" level=info msg=" ====== CHECKING ENDPOINT REACHABILITY USING PLAIN HTTP TRANSPORT ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:17+02:00" level=info msg=" ====== ENDPOINT REACHABILITY USING PLAIN HTTP TRANSPORT SUCCEED ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:17+02:00" level=info msg=" ====== CHECKING ENDPOINT REACHABILITY USING PLAIN HEAD REQUEST ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:18+02:00" level=info msg=" ====== ENDPOINT REACHABILITY USING PLAIN HEAD REQUEST SUCCEED ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:18+02:00" level=info msg=" ====== CHECKING ENDPOINT REACHABILITY USING PUBLIC DNS SERVER ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:18+02:00" level=info msg=" ====== ENDPOINT REACHABILITY USING PUBLIC DNS SERVER SUCCEED ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:18+02:00" level=info msg=" ====== CHECKING ENDPOINT REACHABILITY USING GOLANG DNS CUSTOM RESOLVER ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:19+02:00" level=info msg=" ====== ENDPOINT REACHABILITY USING GOLANG DNS CUSTOM RESOLVER SUCCEED ====== " component=AgentService service=newrelic-infra
time="2023-10-04T17:04:19+02:00" level=info msg=Initializing component=AgentService elapsedTime=3.976758125s service=newrelic-infra version=
```
