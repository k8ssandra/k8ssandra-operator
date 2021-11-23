### Generating mocks

Mocks in this package were generated using Mockery.

To install Mockery on macOS:

    brew install mockery

If necessary, mocks can be regenerated with:

    mockery --dir=./pkg/cassandra --output=./pkg/mocks --name=ManagementApiFacade
    mockery --dir=./pkg/reaper --output=./pkg/mocks --name=Manager  --filename=reaper_manager.go --structname=ReaperManager
