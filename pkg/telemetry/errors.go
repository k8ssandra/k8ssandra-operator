package telemetry

type TelemetryConfigIncomplete struct{ missingfield string }

func (detail TelemetryConfigIncomplete) Error() string {
	return "TelemetryConfig did not contain required fields to process into a TelemetryConfig, missing field " + detail.missingfield
}

type TelemetryDepsNotInstalled struct{ missingdeps string }

func (detail TelemetryDepsNotInstalled) Error() string {
	return "prerequisite telemetry components were not found, missing deps are " + detail.missingdeps
}
