module fake-time-injector/faketimeimg

go 1.18

require (
	github.com/chaos-mesh/chaos-mesh v0.9.1-0.20220812140450-4bc7ef589c13
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/google/uuid v1.2.0
	github.com/pingcap/failpoint v0.0.0-20200210140405-f8f9fb234798
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	go.uber.org/zap v1.21.0
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.28.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)

// github.com/chaos-mesh/chaos-mesh require /api/v1alpha1 v0.0.0, but v0.0.0 can not be found, so use replace here
replace (
	github.com/chaos-mesh/chaos-mesh v0.9.1-0.20220812140450-4bc7ef589c13 => ./
	github.com/chaos-mesh/chaos-mesh/api => github.com/chaos-mesh/chaos-mesh/api v0.0.0-20220717162241-8644a0680800
)
