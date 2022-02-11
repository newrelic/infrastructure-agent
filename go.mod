module github.com/newrelic/infrastructure-agent

go 1.16

require (
	github.com/Microsoft/go-winio v0.5.1
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6
	github.com/antihax/optional v1.0.0
	github.com/aws/aws-sdk-go v1.25.14-0.20200515182354-0961961790e6
	github.com/containerd/containerd v1.5.9 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/fortytw2/leaktest v1.3.1-0.20190606143808-d73c753520d9
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.0
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/service v1.1.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kolo/xmlrpc v0.0.0-20200310150728-e0350524596b
	github.com/kr/text v0.2.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/newrelic/go-agent/v3 v3.15.2
	github.com/newrelic/infra-identity-client-go v1.0.2
	github.com/newrelic/newrelic-telemetry-sdk-go v0.8.1
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/procfs v0.6.0
	github.com/shirou/gopsutil/v3 v3.21.11
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tevino/abool v1.2.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.13.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	golang.org/x/net v0.0.0-20220114011407-0dd24b26b47d
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20220118154757-00ab72f36ad5 // indirect
	google.golang.org/grpc v1.43.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.1-0.20181123051433-bcbf6e613274+incompatible
)

replace (
	// fixing CVEs
	github.com/containerd/containerd v1.2.10 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.3.0 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.3.0-beta.2.0.20190828155532-0293cbd26c69 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.3.1-0.20191213020239-082f7e3aed57 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.3.2 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.4.0-beta.2.0.20200729163537-40b22ef07410 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.4.1 => github.com/containerd/containerd v1.4.8
	github.com/containerd/containerd v1.4.3 => github.com/containerd/containerd v1.4.8
	github.com/coreos/go-systemd/v22 v22.1.0 => github.com/newrelic-forks/go-systemd/v22 v22.1.1
	github.com/kardianos/service v1.1.0 => github.com/newrelic-forks/service v1.1.2
)
