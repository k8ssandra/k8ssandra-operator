package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_computeNodeAffinityLabels(t *testing.T) {

	t.Run("no labels", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{}, "default")
		assert.Len(t, actual, 0)
	})

	t.Run("non existent rack", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
			}}, "nonexistent")
		require.Nil(t, actual)
	})

	t.Run("affinity on dc label", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
			}}, "default")
		require.NotNil(t, actual)
		assert.Equal(t, "value1", actual["label1"])
	})

	t.Run("affinity on dc label with rack", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
				Racks:              []cassdcapi.Rack{{Name: "rack1"}},
			}}, "rack1")
		require.NotNil(t, actual)
		assert.Equal(t, "value1", actual["label1"])
	})

	t.Run("affinity on rack label", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				Racks: []cassdcapi.Rack{{
					Name:               "rack1",
					NodeAffinityLabels: map[string]string{"label1": "value1"},
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		assert.Equal(t, "value1", actual["label1"])
	})

	t.Run("affinity on dc and rack labels", func(t *testing.T) {
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
				Racks: []cassdcapi.Rack{{
					Name:               "rack1",
					NodeAffinityLabels: map[string]string{"label2": "value2"},
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		assert.Equal(t, "value1", actual["label1"])
		assert.Equal(t, "value2", actual["label2"])
	})

	t.Run("affinity on zone label", func(t *testing.T) {
		//goland:noinspection GoDeprecation
		actual := computeNodeAffinityLabels(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				Racks: []cassdcapi.Rack{{
					Name: "rack1",
					Zone: "zone1",
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		assert.Equal(t, "zone1", actual[zoneLabel])
	})
}

func Test_computeNodeAffinity(t *testing.T) {

	t.Run("no labels", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{}, "default")
		assert.Nil(t, actual)
	})

	t.Run("non existent rack", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
			}}, "nonexistent")
		require.Nil(t, actual)
	})

	t.Run("affinity on dc label", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
			}}, "default")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		assert.Equal(t, "label1", actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key)
		assert.Equal(t, "value1", actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0])
	})

	t.Run("affinity on dc label with rack", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
				Racks:              []cassdcapi.Rack{{Name: "rack1"}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		requirement := actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]
		assert.Equal(t, "label1", requirement.Key)
		assert.Equal(t, "value1", requirement.Values[0])
	})

	t.Run("affinity on rack label", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				Racks: []cassdcapi.Rack{{
					Name:               "rack1",
					NodeAffinityLabels: map[string]string{"label1": "value1"},
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		requirement := actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]
		assert.Equal(t, "label1", requirement.Key)
		assert.Equal(t, "value1", requirement.Values[0])
	})

	t.Run("affinity on dc and rack labels", func(t *testing.T) {
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{"label1": "value1"},
				Racks: []cassdcapi.Rack{{
					Name:               "rack1",
					NodeAffinityLabels: map[string]string{"label2": "value2"},
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		requirement1 := actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]
		assert.Equal(t, "label1", requirement1.Key)
		assert.Equal(t, "value1", requirement1.Values[0])
		requirement2 := actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[1]
		assert.Equal(t, "label2", requirement2.Key)
		assert.Equal(t, "value2", requirement2.Values[0])
	})

	t.Run("affinity on zone label", func(t *testing.T) {
		//goland:noinspection GoDeprecation
		actual := computeNodeAffinity(&cassdcapi.CassandraDatacenter{
			Spec: cassdcapi.CassandraDatacenterSpec{
				NodeAffinityLabels: map[string]string{zoneLabel: "zone1"},
				Racks: []cassdcapi.Rack{{
					Name: "rack1",
					Zone: "zone1",
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		assert.Equal(t, zoneLabel, actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key)
		assert.Equal(t, "zone1", actual.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0])
	})

}

func Test_computePodAntiAffinity(t *testing.T) {

	t.Run("multiple workers forbidden", func(t *testing.T) {
		//goland:noinspection GoDeprecation
		actual := computePodAntiAffinity(false, &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dc1",
				Namespace: "namespace1",
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName: "cluster1",
				Racks: []cassdcapi.Rack{{
					Name: "rack1",
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.NotNil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		require.Nil(t, actual.PreferredDuringSchedulingIgnoredDuringExecution)
		selector := actual.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector
		assert.Equal(t, cassdcapi.ClusterLabel, selector.MatchExpressions[0].Key)
		assert.Equal(t, "cluster1", selector.MatchExpressions[0].Values[0])
		assert.Equal(t, cassdcapi.DatacenterLabel, selector.MatchExpressions[1].Key)
		assert.Equal(t, "dc1", selector.MatchExpressions[1].Values[0])
		assert.Equal(t, cassdcapi.RackLabel, selector.MatchExpressions[2].Key)
		assert.Equal(t, "rack1", selector.MatchExpressions[2].Values[0])
	})

	t.Run("multiple workers allowed", func(t *testing.T) {
		//goland:noinspection GoDeprecation
		actual := computePodAntiAffinity(true, &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dc1",
				Namespace: "namespace1",
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				ClusterName: "cluster1",
				Racks: []cassdcapi.Rack{{
					Name: "rack1",
				}},
			}}, "rack1")
		require.NotNil(t, actual)
		require.Nil(t, actual.RequiredDuringSchedulingIgnoredDuringExecution)
		require.NotNil(t, actual.PreferredDuringSchedulingIgnoredDuringExecution)
		selector := actual.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector
		assert.Equal(t, cassdcapi.ClusterLabel, selector.MatchExpressions[0].Key)
		assert.Equal(t, "cluster1", selector.MatchExpressions[0].Values[0])
		assert.Equal(t, cassdcapi.DatacenterLabel, selector.MatchExpressions[1].Key)
		assert.Equal(t, "dc1", selector.MatchExpressions[1].Values[0])
		assert.Equal(t, cassdcapi.RackLabel, selector.MatchExpressions[2].Key)
		assert.Equal(t, "rack1", selector.MatchExpressions[2].Values[0])
	})
}
