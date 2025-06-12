module github.com/k8ssandra/k8ssandra-operator

go 1.23.0

toolchain go1.24.2

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/adutra/goalesce v0.0.0-20221124153206-5643f911003d
	github.com/apache/tinkerpop/gremlin-go v0.0.0-20220530191148-29272fa563ec
	github.com/bombsimon/logrusr/v2 v2.0.1
	github.com/cert-manager/cert-manager v1.10.2
	github.com/datastax/go-cassandra-native-protocol v0.0.0-20220706104457-5e8aad05cf90
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/go-logr/logr v1.4.2
	github.com/go-logr/zapr v1.3.0
	github.com/google/uuid v1.6.0
	github.com/gruntwork-io/terratest v0.48.2
	github.com/k8ssandra/cass-operator v1.25.0
	github.com/k8ssandra/reaper-client-go v0.3.1-0.20220114183114-6923e077c4f5
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.52.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/zerolog v1.20.0
	github.com/sirupsen/logrus v1.9.3
	github.com/square/certigo v1.16.0
	github.com/stargate/stargate-grpc-go-client v0.0.0-20220822130422-9a1c6261d4fa
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.31.7
	k8s.io/apimachinery v0.31.7
	k8s.io/client-go v0.31.7
	k8s.io/utils v0.0.0-20250321185631-1f6e0b77f77e
	sigs.k8s.io/controller-runtime v0.19.7
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/Jeffail/gabs/v2 v2.7.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.2.0 // indirect
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/oauth2 v0.29.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241104194629-dd2ea8efbc28 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.31.7 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	sigs.k8s.io/gateway-api v0.5.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.31.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.31.7
	k8s.io/apiserver => k8s.io/apiserver v0.31.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.7
	k8s.io/client-go => k8s.io/client-go v0.31.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.31.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.31.7
	k8s.io/code-generator => k8s.io/code-generator v0.31.7
	k8s.io/component-base => k8s.io/component-base v0.31.7
	k8s.io/component-helpers => k8s.io/component-helpers v0.31.7
	k8s.io/controller-manager => k8s.io/controller-manager v0.31.7
	k8s.io/cri-api => k8s.io/cri-api v0.31.7
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.31.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.31.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.31.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.31.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.31.7
	k8s.io/kubectl => k8s.io/kubectl v0.31.7
	k8s.io/kubelet => k8s.io/kubelet v0.31.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.31.7
	k8s.io/metrics => k8s.io/metrics v0.31.7
	k8s.io/mount-utils => k8s.io/mount-utils v0.31.7
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.31.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.31.7
)
