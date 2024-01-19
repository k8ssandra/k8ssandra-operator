package medusa

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMedusaIni(t *testing.T) {
	t.Run("Full", testMedusaIniFull)
	t.Run("NoPrefix", testMedusaIniNoPrefix)
	t.Run("Secured", testMedusaIniSecured)
	t.Run("Unsecured", testMedusaIniUnsecured)
	t.Run("MissingOptional", testMedusaIniMissingOptionalSettings)
}

func testMedusaIniFull(t *testing.T) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "k8sCtx0",
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
						},
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "s3",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					BucketName:               "bucket",
					Prefix:                   "prefix",
					MaxBackupAge:             10,
					MaxBackupCount:           20,
					ApiProfile:               "default",
					TransferMaxBandwidth:     "100MB/s",
					ConcurrentTransfers:      2,
					MultiPartUploadThreshold: 204857600,
					Host:                     "192.168.0.1",
					Region:                   "us-east-1",
					Port:                     9001,
					Secure:                   false,
					BackupGracePeriodInDays:  7,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: "test-superuser",
				},
			},
		},
	}

	medusaIni := CreateMedusaIni(kc)

	assert.Contains(t, medusaIni, "storage_provider = s3")
	assert.Contains(t, medusaIni, "bucket_name = bucket")
	assert.Contains(t, medusaIni, "prefix = prefix")
	assert.Contains(t, medusaIni, "max_backup_age = 10")
	assert.Contains(t, medusaIni, "max_backup_count = 20")
	assert.Contains(t, medusaIni, "api_profile = default")
	assert.Contains(t, medusaIni, "transfer_max_bandwidth = 100MB/s")
	assert.Contains(t, medusaIni, "concurrent_transfers = 2")
	assert.Contains(t, medusaIni, "multi_part_upload_threshold = 204857600")
	assert.Contains(t, medusaIni, "host = 192.168.0.1")
	assert.Contains(t, medusaIni, "region = us-east-1")
	assert.Contains(t, medusaIni, "port = 9001")
	assert.Contains(t, medusaIni, "secure = False")
	assert.Contains(t, medusaIni, "backup_grace_period_in_days = 7")
}

func testMedusaIniNoPrefix(t *testing.T) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "demo",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "k8sCtx0",
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
						},
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "s3",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					BucketName:               "bucket",
					MaxBackupAge:             10,
					MaxBackupCount:           20,
					ApiProfile:               "default",
					TransferMaxBandwidth:     "100MB/s",
					ConcurrentTransfers:      2,
					MultiPartUploadThreshold: 204857600,
					Host:                     "192.168.0.1",
					Region:                   "us-east-1",
					Port:                     9001,
					Secure:                   false,
					BackupGracePeriodInDays:  7,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: "test-superuser",
				},
			},
		},
	}

	medusaIni := CreateMedusaIni(kc)
	assert.Contains(t, medusaIni, "storage_provider = s3")
	assert.Contains(t, medusaIni, "bucket_name = bucket")
	assert.Contains(t, medusaIni, "prefix = demo")
	assert.Contains(t, medusaIni, "max_backup_age = 10")
	assert.Contains(t, medusaIni, "max_backup_count = 20")
	assert.Contains(t, medusaIni, "api_profile = default")
	assert.Contains(t, medusaIni, "transfer_max_bandwidth = 100MB/s")
	assert.Contains(t, medusaIni, "concurrent_transfers = 2")
	assert.Contains(t, medusaIni, "multi_part_upload_threshold = 204857600")
	assert.Contains(t, medusaIni, "host = 192.168.0.1")
	assert.Contains(t, medusaIni, "region = us-east-1")
	assert.Contains(t, medusaIni, "port = 9001")
	assert.Contains(t, medusaIni, "secure = False")
	assert.Contains(t, medusaIni, "backup_grace_period_in_days = 7")
}

func testMedusaIniSecured(t *testing.T) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "demo",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "k8sCtx0",
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
						},
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "s3",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					BucketName:               "bucket",
					MaxBackupAge:             10,
					MaxBackupCount:           20,
					ApiProfile:               "default",
					TransferMaxBandwidth:     "100MB/s",
					ConcurrentTransfers:      2,
					MultiPartUploadThreshold: 204857600,
					Host:                     "192.168.0.1",
					Region:                   "us-east-1",
					Port:                     9001,
					Secure:                   true,
					SslVerify:                true,
					BackupGracePeriodInDays:  7,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: "test-superuser",
				},
			},
		},
	}

	medusaIni := CreateMedusaIni(kc)
	assert.Contains(t, medusaIni, "storage_provider = s3")
	assert.Contains(t, medusaIni, "bucket_name = bucket")
	assert.Contains(t, medusaIni, "prefix = demo")
	assert.Contains(t, medusaIni, "max_backup_age = 10")
	assert.Contains(t, medusaIni, "max_backup_count = 20")
	assert.Contains(t, medusaIni, "api_profile = default")
	assert.Contains(t, medusaIni, "transfer_max_bandwidth = 100MB/s")
	assert.Contains(t, medusaIni, "concurrent_transfers = 2")
	assert.Contains(t, medusaIni, "multi_part_upload_threshold = 204857600")
	assert.Contains(t, medusaIni, "host = 192.168.0.1")
	assert.Contains(t, medusaIni, "region = us-east-1")
	assert.Contains(t, medusaIni, "port = 9001")
	assert.Contains(t, medusaIni, "secure = True")
	assert.Contains(t, medusaIni, "ssl_verify = True")
	assert.Contains(t, medusaIni, "backup_grace_period_in_days = 7")
}

func testMedusaIniUnsecured(t *testing.T) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "demo",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "k8sCtx0",
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
						},
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "s3",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					BucketName:               "bucket",
					MaxBackupAge:             10,
					MaxBackupCount:           20,
					ApiProfile:               "default",
					TransferMaxBandwidth:     "100MB/s",
					ConcurrentTransfers:      2,
					MultiPartUploadThreshold: 204857600,
					Host:                     "192.168.0.1",
					Region:                   "us-east-1",
					Port:                     9001,
					Secure:                   true,
					BackupGracePeriodInDays:  7,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: "test-superuser",
				},
			},
		},
	}

	medusaIni := CreateMedusaIni(kc)
	assert.Contains(t, medusaIni, "storage_provider = s3")
	assert.Contains(t, medusaIni, "bucket_name = bucket")
	assert.Contains(t, medusaIni, "prefix = demo")
	assert.Contains(t, medusaIni, "max_backup_age = 10")
	assert.Contains(t, medusaIni, "max_backup_count = 20")
	assert.Contains(t, medusaIni, "api_profile = default")
	assert.Contains(t, medusaIni, "transfer_max_bandwidth = 100MB/s")
	assert.Contains(t, medusaIni, "concurrent_transfers = 2")
	assert.Contains(t, medusaIni, "multi_part_upload_threshold = 204857600")
	assert.Contains(t, medusaIni, "host = 192.168.0.1")
	assert.Contains(t, medusaIni, "region = us-east-1")
	assert.Contains(t, medusaIni, "port = 9001")
	assert.Contains(t, medusaIni, "secure = True")
	assert.Contains(t, medusaIni, "ssl_verify = False")
	assert.Contains(t, medusaIni, "backup_grace_period_in_days = 7")
}

func testMedusaIniMissingOptionalSettings(t *testing.T) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "demo",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "k8sCtx0",
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
						},
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "s3",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					BucketName: "bucket",
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: "test-superuser",
				},
			},
		},
	}

	medusaIni := CreateMedusaIni(kc)
	assert.Contains(t, medusaIni, "storage_provider = s3")
	assert.Contains(t, medusaIni, "bucket_name = bucket")
	assert.Contains(t, medusaIni, "prefix = demo")
	assert.Contains(t, medusaIni, "max_backup_age = 0")
	assert.Contains(t, medusaIni, "max_backup_count = 0")
	assert.NotContains(t, medusaIni, "api_profile =")
	assert.NotContains(t, medusaIni, "transfer_max_bandwidth =")
	assert.NotContains(t, medusaIni, "concurrent_transfers =")
	assert.NotContains(t, medusaIni, "multi_part_upload_threshold =")
	assert.NotContains(t, medusaIni, "host =")
	assert.NotContains(t, medusaIni, "region =")
	assert.NotContains(t, medusaIni, "port =")
	assert.Contains(t, medusaIni, "secure = False")
	assert.NotContains(t, medusaIni, "backup_grace_period_in_days =")
}

func TestInitContainerDefaultResources(t *testing.T) {
	medusaSpec := &medusaapi.MedusaClusterTemplate{
		StorageProperties: medusaapi.Storage{
			StorageProvider: "s3",
			StorageSecretRef: corev1.LocalObjectReference{
				Name: "secret",
			},
			BucketName: "bucket",
		},
		CassandraUserSecretRef: corev1.LocalObjectReference{
			Name: "test-superuser",
		},
	}

	dcConfig := cassandra.DatacenterConfig{}

	logger := logr.New(logr.Discard().GetSink())

	medusaContainer, err := CreateMedusaMainContainer(&dcConfig, medusaSpec, false, "test", logger)
	assert.NoError(t, err)
	UpdateMedusaInitContainer(&dcConfig, medusaSpec, false, "test", logger)
	UpdateMedusaMainContainer(&dcConfig, medusaContainer)

	assert.Equal(t, 1, len(dcConfig.PodTemplateSpec.Spec.Containers))
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.InitContainers))
	// Init container resources
	medusaInitContainerIndex, found := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "medusa-restore")
	assert.True(t, found, "Couldn't find medusa-restore init container")

	assert.Equal(t, resource.MustParse(InitContainerMemRequest), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Requests.Memory(), "expected init container memory request to be set")
	assert.Equal(t, resource.MustParse(InitContainerCpuRequest), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Requests.Cpu(), "expected init container cpu request to be set")
	assert.Equal(t, resource.MustParse(InitContainerMemLimit), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Limits.Memory(), "expected init container memory limit to be set")

	// Main container resources
	assert.Equal(t, resource.MustParse(MainContainerMemRequest), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Memory(), "expected main container memory request to be set")
	assert.Equal(t, resource.MustParse(MainContainerCpuRequest), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Cpu(), "expected main container cpu request to be set")
	assert.Equal(t, resource.MustParse(MainContainerMemLimit), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Limits.Memory(), "expected main container memory limit to be set")

}

func TestInitContainerCustomResources(t *testing.T) {
	medusaSpec := &medusaapi.MedusaClusterTemplate{
		StorageProperties: medusaapi.Storage{
			StorageProvider: "s3",
			StorageSecretRef: corev1.LocalObjectReference{
				Name: "secret",
			},
			BucketName: "bucket",
		},
		CassandraUserSecretRef: corev1.LocalObjectReference{
			Name: "test-superuser",
		},
		InitContainerResources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("10Gi"),
				corev1.ResourceCPU:    resource.MustParse("10"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("20Gi"),
				corev1.ResourceCPU:    resource.MustParse("20"),
			},
		},
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("30Gi"),
				corev1.ResourceCPU:    resource.MustParse("30"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("40Gi"),
				corev1.ResourceCPU:    resource.MustParse("40"),
			},
		},
	}

	dcConfig := cassandra.DatacenterConfig{}

	logger := logr.New(logr.Discard().GetSink())

	medusaContainer, err := CreateMedusaMainContainer(&dcConfig, medusaSpec, false, "test", logger)
	assert.NoError(t, err)
	UpdateMedusaInitContainer(&dcConfig, medusaSpec, false, "test", logger)
	UpdateMedusaMainContainer(&dcConfig, medusaContainer)

	assert.Equal(t, 1, len(dcConfig.PodTemplateSpec.Spec.Containers))
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.InitContainers))
	// Init container resources
	medusaInitContainerIndex, found := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "medusa-restore")
	assert.True(t, found, "Couldn't find medusa-restore init container")

	assert.Equal(t, resource.MustParse("10Gi"), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Requests.Memory(), "expected init container memory request to be set")
	assert.Equal(t, resource.MustParse("10"), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Requests.Cpu(), "expected init container cpu request to be set")
	assert.Equal(t, resource.MustParse("20Gi"), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Limits.Memory(), "expected init container memory limit to be set")
	assert.Equal(t, resource.MustParse("20"), *dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Resources.Limits.Cpu(), "expected init container cpu limit to be set")

	// Main container resources
	assert.Equal(t, resource.MustParse("30Gi"), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Memory(), "expected main container memory request to be set")
	assert.Equal(t, resource.MustParse("30"), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Cpu(), "expected main container cpu request to be set")
	assert.Equal(t, resource.MustParse("40Gi"), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Limits.Memory(), "expected main container memory limit to be set")
	assert.Equal(t, resource.MustParse("40"), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Limits.Cpu(), "expected main container cpu limit to be set")

}

func TestExternalSecretsFlag(t *testing.T) {
	medusaSpec := &medusaapi.MedusaClusterTemplate{
		StorageProperties: medusaapi.Storage{
			StorageProvider: "s3",
			StorageSecretRef: corev1.LocalObjectReference{
				Name: "secret",
			},
			BucketName: "bucket",
		},
		CassandraUserSecretRef: corev1.LocalObjectReference{
			Name: "test-superuser",
		},
	}

	dcConfig := cassandra.DatacenterConfig{}

	logger := logr.New(logr.Discard().GetSink())

	medusaContainer, err := CreateMedusaMainContainer(&dcConfig, medusaSpec, true, "test", logger)
	assert.NoError(t, err)
	UpdateMedusaInitContainer(&dcConfig, medusaSpec, true, "test", logger)
	UpdateMedusaMainContainer(&dcConfig, medusaContainer)

	medusaInitContainerIndex, found := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "medusa-restore")
	assert.True(t, found, "Couldn't find medusa-restore init container")

	assert.Equal(t, 3, len(dcConfig.PodTemplateSpec.Spec.Containers[0].Env))
	assert.Equal(t, "MEDUSA_MODE", dcConfig.PodTemplateSpec.Spec.Containers[0].Env[0].Name)
	assert.Equal(t, "MEDUSA_TMP_DIR", dcConfig.PodTemplateSpec.Spec.Containers[0].Env[1].Name)
	assert.Equal(t, "POD_NAME", dcConfig.PodTemplateSpec.Spec.Containers[0].Env[2].Name)

	assert.Equal(t, 3, len(dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Env))
	assert.Equal(t, "MEDUSA_MODE", dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Env[0].Name)
	assert.Equal(t, "MEDUSA_TMP_DIR", dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Env[1].Name)
	assert.Equal(t, "POD_NAME", dcConfig.PodTemplateSpec.Spec.InitContainers[medusaInitContainerIndex].Env[2].Name)
}

func TestGenerateMedusaProbe(t *testing.T) {
	customProbeSettings := &corev1.Probe{
		InitialDelaySeconds: 100,
		TimeoutSeconds:      200,
		PeriodSeconds:       300,
		SuccessThreshold:    400,
		FailureThreshold:    500,
	}

	customProbe, err := generateMedusaProbe(customProbeSettings)
	assert.NoError(t, err)
	assert.Equal(t, int32(100), customProbe.InitialDelaySeconds)
	assert.Equal(t, int32(200), customProbe.TimeoutSeconds)
	assert.Equal(t, int32(300), customProbe.PeriodSeconds)
	assert.Equal(t, int32(400), customProbe.SuccessThreshold)
	assert.Equal(t, int32(500), customProbe.FailureThreshold)

	defaultProbe, err := generateMedusaProbe(nil)
	assert.NoError(t, err)
	assert.Equal(t, int32(DefaultProbeInitialDelay), defaultProbe.InitialDelaySeconds)
	assert.Equal(t, int32(DefaultProbeTimeout), defaultProbe.TimeoutSeconds)
	assert.Equal(t, int32(DefaultProbePeriod), defaultProbe.PeriodSeconds)
	assert.Equal(t, int32(DefaultProbeSuccessThreshold), defaultProbe.SuccessThreshold)
	assert.Equal(t, int32(DefaultProbeFailureThreshold), defaultProbe.FailureThreshold)

	// Test that changing the probe handler is rejected
	rejectedProbe := &corev1.Probe{
		InitialDelaySeconds: 100,
		TimeoutSeconds:      200,
		PeriodSeconds:       300,
		SuccessThreshold:    400,
		FailureThreshold:    500,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
			},
		},
	}
	probe, err := generateMedusaProbe(rejectedProbe)
	assert.Error(t, err)
	assert.Nil(t, probe)
}

func TestPurgeCronJob(t *testing.T) {
	// Define your test inputs
	dcConfig := &cassandra.DatacenterConfig{
		DatacenterName: "testDc",
	}
	clusterName := "testCluster"
	namespace := "testNamespace"
	logger := logr.New(logr.Discard().GetSink())

	// Call the function with the test inputs
	actualCronJob, err := PurgeCronJob(dcConfig, clusterName, namespace, logger)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s-%s-medusa-purge", "testcluster", "testdc"), actualCronJob.ObjectMeta.Name)
	assert.Equal(t, 3, len(actualCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command))
	assert.Contains(t, actualCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command[2], "\\nspec:\\n  cassandraDatacenter: testdc")
}

func TestPurgeCronJobNameTooLong(t *testing.T) {
	// Define your test inputs
	dcConfig := &cassandra.DatacenterConfig{
		DatacenterName: "testDatacentercWithAReallyLongNameToTestThatTheCharacterCountOfTheNameGoesOverTwoHundredFiftyThreeCharactersTestTestTestTest",
	}
	clusterName := "testClusterNameBeingWayTooLongToTestThatTheCharacterCountOfTheNameGoesOverTwoHundredFiftyThreeCharactersTestTestTestTest"
	namespace := "testNamespace"
	logger := logr.New(logr.Discard().GetSink())

	// Call the function with the test inputs
	_, err := PurgeCronJob(dcConfig, clusterName, namespace, logger)
	assert.NotNil(t, err)
}
