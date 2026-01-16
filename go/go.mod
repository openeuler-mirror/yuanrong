module yuanrong.org/kernel

go 1.24.1

require (
	clients v1.0.0
	yuanrong.org/kernel/pkg/common v1.0.0
	github.com/agiledragon/gomonkey/v2 v2.11.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/automaxprocs v1.6.0
	yuanrong.org/kernel/runtime v1.0.0
)

replace (
	clients => ../datasystem/output/sdk/go
	yuanrong.org/kernel/pkg/common => ./pkg/common
	github.com/agiledragon/gomonkey => github.com/agiledragon/gomonkey v2.0.1+incompatible
	github.com/asaskevich/govalidator/v11 => github.com/asaskevich/govalidator/v11 v11.0.1-0.20250122183457-e11347878e23
	github.com/fsnotify/fsnotify => github.com/fsnotify/fsnotify v1.7.0
	// for test or internal use
	github.com/gin-gonic/gin => github.com/gin-gonic/gin v1.10.0
	github.com/golang/mock => github.com/golang/mock v1.3.1
	github.com/google/uuid => github.com/google/uuid v1.6.0
	github.com/olekukonko/tablewriter => github.com/olekukonko/tablewriter v0.0.5
	github.com/operator-framework/operator-lib => github.com/operator-framework/operator-lib v0.4.0
	github.com/prashantv/gostub => github.com/prashantv/gostub v1.0.0
	github.com/robfig/cron/v3 => github.com/robfig/cron/v3 v3.0.1
	github.com/smartystreets/goconvey => github.com/smartystreets/goconvey v1.6.4
	github.com/spf13/cobra => github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify => github.com/stretchr/testify v1.5.1
	github.com/valyala/fasthttp => github.com/valyala/fasthttp v1.58.0
	go.etcd.io/etcd/api/v3 => go.etcd.io/etcd/api/v3 v3.5.11
	go.etcd.io/etcd/client/v3 => go.etcd.io/etcd/client/v3 v3.5.11
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace => go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc => go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.24.0
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.24.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.24.0
	go.uber.org/automaxprocs => go.uber.org/automaxprocs v1.6.0
	go.uber.org/zap => go.uber.org/zap v1.27.0
	golang.org/x/crypto => golang.org/x/crypto v0.24.0
	// affects VPC plugin building, will cause error if not pinned
	golang.org/x/net => golang.org/x/net v0.26.0
	golang.org/x/sync => golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys => golang.org/x/sys v0.21.0
	golang.org/x/text => golang.org/x/text v0.16.0
	golang.org/x/time => golang.org/x/time v0.10.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20230526203410-71b5a4ffd15e
	google.golang.org/genproto/googleapis/rpc => google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d
	google.golang.org/grpc => google.golang.org/grpc v1.67.0
	google.golang.org/protobuf => google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1
	yuanrong.org/kernel/runtime => ../api/go
	k8s.io/api => k8s.io/api v0.31.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.31.2
	k8s.io/client-go => k8s.io/client-go v0.31.2
)
