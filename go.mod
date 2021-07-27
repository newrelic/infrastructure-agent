module github.com/newrelic/infrastructure-agent

go 1.14

require (
	github.com/Microsoft/go-winio v0.5.0
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6
	github.com/antihax/optional v1.0.0
	github.com/aws/aws-sdk-go v1.25.14-0.20200515182354-0961961790e6
	github.com/containerd/containerd v1.5.4 // indirect
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/docker/docker v20.10.7+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/fortytw2/leaktest v1.3.1-0.20190606143808-d73c753520d9
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.0
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/service v1.1.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kolo/xmlrpc v0.0.0-20200310150728-e0350524596b
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/newrelic/infra-identity-client-go v1.0.2
	github.com/opencontainers/image-spec v1.0.2-0.20181029102219-09950c5fb1bb // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/procfs v0.6.0
	github.com/shirou/gopsutil v3.21.6+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.6.1
	github.com/tevino/abool v1.2.0
	github.com/tklauser/go-sysconf v0.3.7 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.13.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	golang.org/x/net v0.0.0-20210726213435-c6fcb2dbf985
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	google.golang.org/genproto v0.0.0-20210726200206-e7812ac95cc0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.1-0.20181123051433-bcbf6e613274+incompatible
)

replace (
	github.com/coreos/go-systemd/v22 v22.1.0 => github.com/newrelic-forks/go-systemd/v22 v22.1.1
	github.com/kardianos/service v1.1.0 => github.com/newrelic-forks/service v1.1.2
)
