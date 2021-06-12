module github.com/k8ssandra/k8ssandra-operator

go 1.16

require (
	github.com/bombsimon/logrusr v1.1.0
	github.com/go-logr/logr v0.3.0
	github.com/k8ssandra/cass-operator v1.7.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/stretchr/testify v1.7.0
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.20.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.20.2
)
