package cassandra

type DCConfigIncomplete struct{ missingField string }

func (detail DCConfigIncomplete) Error() string {
	return "DatacenterConfig did not contain required fields to process into a CassandraDatacenter, missing field " + detail.missingField
}
