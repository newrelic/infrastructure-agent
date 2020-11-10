module github.com/newrelic/infrastructure-agent

go 1.14

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.11
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6
	github.com/antihax/optional v1.0.0
	github.com/aws/aws-sdk-go v1.25.14-0.20200515182354-0961961790e6
	github.com/containerd/containerd v1.3.7 // indirect
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fortytw2/leaktest v1.3.1-0.20190606143808-d73c753520d9
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/gogo/protobuf v1.1.2-0.20181116123445-07eab6a8298c // indirect
	github.com/golang/groupcache v0.0.0-20191027212112-611e8accdfc9
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/mux v1.7.4 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/service v1.1.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kolo/xmlrpc v0.0.0-20200310150728-e0350524596b
	github.com/kr/pretty v0.2.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/newrelic/infra-identity-client-go v1.0.2
	github.com/opencontainers/go-digest v1.0.0-rc1.0.20180430190053-c9281466c8b2 // indirect
	github.com/opencontainers/image-spec v1.0.2-0.20181029102219-09950c5fb1bb // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/procfs v0.2.0
	github.com/shirou/gopsutil v2.18.12-0.20181220224138-a5ace91ccec8+incompatible
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/sirupsen/logrus v1.6.1-0.20200528085638-6699a89a232f
	github.com/stretchr/testify v1.6.1
	github.com/tevino/abool v1.2.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
	golang.org/x/text v0.3.3-0.20190829152558-3d0f7978add9 // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	google.golang.org/grpc v1.29.1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/yaml.v2 v2.2.7
	gotest.tools v2.2.1-0.20181123051433-bcbf6e613274+incompatible
)

replace (
	github.com/coreos/go-systemd/v22 v22.1.0 => github.com/newrelic-forks/go-systemd/v22 v22.1.1
	github.com/kardianos/service v1.1.0 => github.com/newrelic-forks/service v1.1.2
)
