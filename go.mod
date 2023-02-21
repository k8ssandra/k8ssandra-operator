module github.com/k8ssandra/k8ssandra-operator

go 1.19

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/adutra/goalesce v0.0.0-20221124153206-5643f911003d
	github.com/apache/tinkerpop/gremlin-go v0.0.0-20220530191148-29272fa563ec
	github.com/bombsimon/logrusr/v2 v2.0.1
	github.com/datastax/go-cassandra-native-protocol v0.0.0-20220706104457-5e8aad05cf90
	github.com/davecgh/go-spew v1.1.1
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zapr v1.2.3
	github.com/google/uuid v1.3.0
	github.com/gruntwork-io/terratest v0.37.7
	github.com/k8ssandra/cass-operator v1.14.0
	github.com/k8ssandra/reaper-client-go v0.3.1-0.20220114183114-6923e077c4f5
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.52.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/zerolog v1.20.0
	github.com/sirupsen/logrus v1.8.1
	github.com/square/certigo v1.16.0
	github.com/stargate/stargate-grpc-go-client v0.0.0-20220822130422-9a1c6261d4fa
	github.com/stretchr/testify v1.8.0
	go.uber.org/zap v1.21.0
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.28.0
	gopkg.in/resty.v1 v1.12.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.25.3
	k8s.io/apimachinery v0.25.3
	k8s.io/client-go v0.25.3
	k8s.io/utils v0.0.0-20220728103510-ee6ede2d64ed
	sigs.k8s.io/controller-runtime v0.13.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	cloud.google.com/go v0.99.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.27 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.20 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/Jeffail/gabs/v2 v2.6.1 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/containerd v1.6.6 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.17+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.8.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.2.0 // indirect
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799 // indirect
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.0.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.12.2 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.4.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/term v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.25.3 // indirect
	k8s.io/component-base v0.25.3 // indirect
	k8s.io/klog/v2 v2.70.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220803162953-67bda5d908f1 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.25.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.25.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.25.3
	k8s.io/apiserver => k8s.io/apiserver v0.25.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.25.3
	k8s.io/client-go => k8s.io/client-go v0.25.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.25.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.25.3
	k8s.io/code-generator => k8s.io/code-generator v0.25.3
	k8s.io/component-base => k8s.io/component-base v0.25.3
	k8s.io/component-helpers => k8s.io/component-helpers v0.25.3
	k8s.io/controller-manager => k8s.io/controller-manager v0.25.3
	k8s.io/cri-api => k8s.io/cri-api v0.25.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.25.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.25.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.25.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.25.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.25.3
	k8s.io/kubectl => k8s.io/kubectl v0.25.3
	k8s.io/kubelet => k8s.io/kubelet v0.25.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.25.3
	k8s.io/metrics => k8s.io/metrics v0.25.3
	k8s.io/mount-utils => k8s.io/mount-utils v0.25.3
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.25.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.25.3
)
