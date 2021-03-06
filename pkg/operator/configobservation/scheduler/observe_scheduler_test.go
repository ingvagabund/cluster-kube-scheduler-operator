package scheduler

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/cluster-kube-scheduler-operator/pkg/operator/configobservation"
	"github.com/openshift/cluster-kube-scheduler-operator/pkg/operator/operatorclient"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
)

func TestObserveSchedulerConfig(t *testing.T) {
	configMapName := "policy-configmap"
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	tests := []struct {
		description   string
		configMapName string
		updateName    bool
	}{
		{
			description:   "A different configmap name but still we need to set the policy configmap to hardcoded value",
			configMapName: "test-abc",
			updateName:    false,
		},
		{
			description:   "A configmap with same name as policy-configmap but still we need to set the policy configmap to hardcoded value",
			configMapName: "policy-configmap",
			updateName:    false,
		},
		{
			description:   "An empty configmap name should clear anything currently set in the observed config",
			configMapName: "policy-configmap",
			updateName:    true,
		},
	}
	for _, test := range tests {
		if err := indexer.Add(&configv1.Scheduler{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.SchedulerSpec{
				Policy: configv1.ConfigMapNameReference{Name: test.configMapName},
			},
		}); err != nil {
			t.Fatal(err.Error())
		}
		synced := map[string]string{}
		listers := configobservation.Listers{
			SchedulerLister: configlistersv1.NewSchedulerLister(indexer),
			ResourceSync:    &mockResourceSyncer{t: t, synced: synced},
		}
		result, errors := ObserveSchedulerConfig(listers, events.NewInMemoryRecorder("scheduler"), map[string]interface{}{})
		if len(errors) > 0 {
			t.Fatalf("expected len(errors) == 0")
		}
		observedConfigMapName, _, err := unstructured.NestedString(result, "algorithmSource", "policy", "configMap", "name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		observedConfigMapNamespace, _, err := unstructured.NestedString(result, "algorithmSource", "policy", "configMap", "namespace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if observedConfigMapName != configMapName {
			t.Fatalf("expected configmap to be %v but got %v for %v", observedConfigMapName, configMapName, test.description)
		}
		if observedConfigMapNamespace != operatorclient.TargetNamespace {
			t.Fatalf("expected target namespace to be %v but got %v for %v", observedConfigMapName, configMapName, test.description)
		}
		if !test.updateName {
			continue
		}

		// clear the configmap name in scheduler config to test that this also carries to the observed config
		if err := indexer.Update(&configv1.Scheduler{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.SchedulerSpec{
				Policy: configv1.ConfigMapNameReference{Name: ""},
			},
		}); err != nil {
			t.Fatal(err.Error())
		}
		result, errors = ObserveSchedulerConfig(listers, events.NewInMemoryRecorder("scheduler"), map[string]interface{}{})
		if len(errors) > 0 {
			t.Fatalf("expected len(errors) == 0")
		}
		source, found, err := unstructured.NestedString(result, "algorithmSource")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatalf("expected observed algorithmSource to be nil, was %v for %v", source, test.description)
		}
	}
}

type mockResourceSyncer struct {
	t      *testing.T
	synced map[string]string
}

func (rs *mockResourceSyncer) SyncConfigMap(destination, source resourcesynccontroller.ResourceLocation) error {
	if (source == resourcesynccontroller.ResourceLocation{}) {
		rs.synced[fmt.Sprintf("configmap/%v.%v", destination.Name, destination.Namespace)] = "DELETE"
	} else {
		rs.synced[fmt.Sprintf("configmap/%v.%v", destination.Name, destination.Namespace)] = fmt.Sprintf("configmap/%v.%v", source.Name, source.Namespace)
	}
	return nil
}

func (rs *mockResourceSyncer) SyncSecret(destination, source resourcesynccontroller.ResourceLocation) error {
	if (source == resourcesynccontroller.ResourceLocation{}) {
		rs.synced[fmt.Sprintf("secret/%v.%v", destination.Name, destination.Namespace)] = "DELETE"
	} else {
		rs.synced[fmt.Sprintf("secret/%v.%v", destination.Name, destination.Namespace)] = fmt.Sprintf("secret/%v.%v", source.Name, source.Namespace)
	}
	return nil
}
