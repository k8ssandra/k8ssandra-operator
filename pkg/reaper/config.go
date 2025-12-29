package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"gopkg.in/yaml.v2"
)

// ReaperConfig represents the complete Reaper configuration structure
type ReaperConfig struct {
	SegmentCountPerNode                    int                  `yaml:"segmentCountPerNode"`
	RepairParallelism                      string               `yaml:"repairParallelism"`
	RepairIntensity                        float64              `yaml:"repairIntensity"`
	MaxPendingCompactions                  int                  `yaml:"maxPendingCompactions"`
	ScheduleDaysBetween                    int                  `yaml:"scheduleDaysBetween"`
	RepairRunThreadCount                   int                  `yaml:"repairRunThreadCount"`
	HangingRepairTimeoutMins               int                  `yaml:"hangingRepairTimeoutMins"`
	StorageType                            string               `yaml:"storageType"`
	EnableCrossOrigin                      bool                 `yaml:"enableCrossOrigin"`
	IncrementalRepair                      bool                 `yaml:"incrementalRepair"`
	SubrangeIncrementalRepair              bool                 `yaml:"subrangeIncrementalRepair"`
	BlacklistTwcsTables                    bool                 `yaml:"blacklistTwcsTables"`
	EnableDynamicSeedList                  bool                 `yaml:"enableDynamicSeedList"`
	RepairManagerSchedulingIntervalSeconds int                  `yaml:"repairManagerSchedulingIntervalSeconds"`
	JmxConnectionTimeoutInSeconds          int                  `yaml:"jmxConnectionTimeoutInSeconds"`
	UseAddressTranslator                   bool                 `yaml:"useAddressTranslator"`
	MaxParallelRepairs                     int                  `yaml:"maxParallelRepairs"`
	ScheduleRetryOnError                   bool                 `yaml:"scheduleRetryOnError"`
	ScheduleRetryDelay                     string               `yaml:"scheduleRetryDelay"`
	PurgeRecordsAfterInDays                int                  `yaml:"purgeRecordsAfterInDays"`
	DatacenterAvailability                 string               `yaml:"datacenterAvailability"`
	AutoScheduling                         AutoSchedulingConfig `yaml:"autoScheduling"`
	JmxPorts                               map[string]int       `yaml:"jmxPorts"`
	Logging                                LoggingConfig        `yaml:"logging"`
	Server                                 ServerConfig         `yaml:"server"`
	HttpManagement                         HttpManagementConfig `yaml:"httpManagement"`
	AccessControl                          AccessControlConfig  `yaml:"accessControl"`
}

type AutoSchedulingConfig struct {
	Enabled                    bool     `yaml:"enabled"`
	InitialDelayPeriod         string   `yaml:"initialDelayPeriod,omitempty"`
	PeriodBetweenPolls         string   `yaml:"periodBetweenPolls,omitempty"`
	TimeBeforeFirstSchedule    string   `yaml:"timeBeforeFirstSchedule,omitempty"`
	ScheduleSpreadPeriod       string   `yaml:"scheduleSpreadPeriod,omitempty"`
	Adaptive                   bool     `yaml:"adaptive,omitempty"`
	Incremental                bool     `yaml:"incremental,omitempty"`
	PercentUnrepairedThreshold int      `yaml:"percentUnrepairedThreshold,omitempty"`
	ExcludedKeyspaces          []string `yaml:"excludedKeyspaces,omitempty"`
	ExcludedClusters           []string `yaml:"excludedClusters,omitempty"`
}

type LoggingConfig struct {
	Level     string                  `yaml:"level"`
	Loggers   map[string]string       `yaml:"loggers"`
	Appenders []LoggingAppenderConfig `yaml:"appenders"`
}

type LoggingAppenderConfig struct {
	Type      string `yaml:"type"`
	LogFormat string `yaml:"logFormat"`
	Threshold string `yaml:"threshold"`
}

type ServerConfig struct {
	Type                  string            `yaml:"type"`
	ApplicationConnectors []ConnectorConfig `yaml:"applicationConnectors"`
	AdminConnectors       []ConnectorConfig `yaml:"adminConnectors"`
	RequestLog            RequestLogConfig  `yaml:"requestLog"`
}

type ConnectorConfig struct {
	Type     string `yaml:"type"`
	Port     int    `yaml:"port"`
	BindHost string `yaml:"bindHost"`
}

type RequestLogConfig struct {
	Appenders []interface{} `yaml:"appenders"`
}

type HttpManagementConfig struct {
	Enabled            bool   `yaml:"enabled"`
	MgmtApiMetricsPort int    `yaml:"mgmtApiMetricsPort,omitempty"`
	Keystore           string `yaml:"keystore,omitempty"`
	Truststore         string `yaml:"truststore,omitempty"`
	TruststoresDir     string `yaml:"truststoresDir,omitempty"`
}

type AccessControlConfig struct {
	Enabled        string       `yaml:"enabled"`
	SessionTimeout string       `yaml:"sessionTimeout"`
	JWT            JWTConfig    `yaml:"jwt"`
	Users          []UserConfig `yaml:"users"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type UserConfig struct {
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	Roles    []string `yaml:"roles"`
}

// computeConfigYAML generates the complete Reaper configuration YAML content
// This replaces the template-based approach where environment variables were substituted
func computeConfigYAML(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter) (string, error) {
	// Determine storage type
	storageType := "cassandra"
	if reaper.Spec.StorageType == api.StorageTypeLocal {
		storageType = "memory"
	}

	// Build auto-scheduling config
	autoScheduling := AutoSchedulingConfig{
		Enabled: reaper.Spec.AutoScheduling.Enabled,
	}

	if reaper.Spec.AutoScheduling.Enabled {
		serverVersion := ""
		if dc != nil && dc.Spec.ServerVersion != "" {
			serverVersion = dc.Spec.ServerVersion
		}
		adaptive, incremental := getAdaptiveIncremental(reaper, serverVersion)

		autoScheduling.InitialDelayPeriod = reaper.Spec.AutoScheduling.InitialDelay
		autoScheduling.PeriodBetweenPolls = reaper.Spec.AutoScheduling.PeriodBetweenPolls
		autoScheduling.TimeBeforeFirstSchedule = reaper.Spec.AutoScheduling.TimeBeforeFirstSchedule
		autoScheduling.ScheduleSpreadPeriod = reaper.Spec.AutoScheduling.ScheduleSpreadPeriod
		autoScheduling.Adaptive = adaptive
		autoScheduling.Incremental = incremental
		autoScheduling.PercentUnrepairedThreshold = reaper.Spec.AutoScheduling.PercentUnrepairedThreshold
		autoScheduling.ExcludedKeyspaces = reaper.Spec.AutoScheduling.ExcludedKeyspaces
		autoScheduling.ExcludedClusters = reaper.Spec.AutoScheduling.ExcludedClusters
	}

	// Build HTTP management config
	httpManagement := HttpManagementConfig{
		Enabled: reaper.Spec.HttpManagement.Enabled,
	}

	if reaper.Spec.HttpManagement.Enabled {
		httpManagement.MgmtApiMetricsPort = 8081
		if reaper.Spec.HttpManagement.Keystores != nil {
			httpManagement.Keystore = "/etc/encryption/mgmt/keystore.jks"
			httpManagement.Truststore = "/etc/encryption/mgmt/truststore.jks"
			httpManagement.TruststoresDir = "/etc/encryption/mgmt"
		}
	}

	// Build the complete config
	config := ReaperConfig{
		SegmentCountPerNode:                    16,
		RepairParallelism:                      "PARALLEL",
		RepairIntensity:                        0.9,
		MaxPendingCompactions:                  20,
		ScheduleDaysBetween:                    7,
		RepairRunThreadCount:                   15,
		HangingRepairTimeoutMins:               30,
		StorageType:                            storageType,
		EnableCrossOrigin:                      true,
		IncrementalRepair:                      false,
		SubrangeIncrementalRepair:              false,
		BlacklistTwcsTables:                    false,
		EnableDynamicSeedList:                  false,
		RepairManagerSchedulingIntervalSeconds: 30,
		JmxConnectionTimeoutInSeconds:          20,
		UseAddressTranslator:                   false,
		MaxParallelRepairs:                     2,
		ScheduleRetryOnError:                   false,
		ScheduleRetryDelay:                     "PT1H",
		PurgeRecordsAfterInDays:                15,
		DatacenterAvailability:                 reaper.Spec.DatacenterAvailability,
		AutoScheduling:                         autoScheduling,
		JmxPorts:                               make(map[string]int),
		Logging: LoggingConfig{
			Level: "INFO",
			Loggers: map[string]string{
				"io.cassandrareaper": "INFO",
			},
			Appenders: []LoggingAppenderConfig{
				{
					Type:      "console",
					LogFormat: "%-6level [%d] [%t] %logger{5} - %msg %n",
					Threshold: "INFO",
				},
			},
		},
		Server: ServerConfig{
			Type: "default",
			ApplicationConnectors: []ConnectorConfig{
				{
					Type:     "http",
					Port:     8080,
					BindHost: "0.0.0.0",
				},
			},
			AdminConnectors: []ConnectorConfig{
				{
					Type:     "http",
					Port:     8081,
					BindHost: "0.0.0.0",
				},
			},
			RequestLog: RequestLogConfig{
				Appenders: []interface{}{},
			},
		},
		HttpManagement: httpManagement,

		AccessControl: AccessControlConfig{
			Enabled:        "${REAPER_AUTH_ENABLED}",
			SessionTimeout: "PT10M",

			// TODO These env variables below were never generated in k8ssandra-operator. Could they be removed entirely from the generated YAML?
			JWT: JWTConfig{
				Secret: "${JWT_SECRET:-MySecretKeyForJWTWhichMustBeLongEnoughForHS256Algorithm}",
			},
			Users: []UserConfig{
				{
					Username: "${REAPER_AUTH_USER}",
					Password: "${REAPER_AUTH_PASSWORD}",
					Roles:    []string{"operator"},
				},
			},
		},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}
