package predicate

import (
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

func NewExcludeNamespacePredicate() predicate.Predicate {
	var excludeWatchNamespaceEnvVar = "EXCLUDE_WATCH_NAMESPACE"

	excludeWatchNamespace, found := os.LookupEnv(excludeWatchNamespaceEnvVar)
	if !found {
		return predicate.NewPredicateFuncs(func(object client.Object) bool {
			return true
		})
	}
	var excludeNamespacesList []string
	if strings.Contains(excludeWatchNamespace, ",") {
		excludeNamespacesList = strings.Split(excludeWatchNamespace, ",")
	} else {
		excludeNamespacesList = []string{excludeWatchNamespace}
	}

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return shouldBeProcessed(excludeNamespacesList, e.ObjectNew.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return shouldBeProcessed(excludeNamespacesList, e.Object.GetNamespace())
		},
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return shouldBeProcessed(excludeNamespacesList, createEvent.Object.GetNamespace())
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return shouldBeProcessed(excludeNamespacesList, genericEvent.Object.GetNamespace())
		},
	}
}

func shouldBeProcessed(excludeNamespacesList []string, namespace string) bool {
	for _, excludeNs := range excludeNamespacesList {
		if excludeNs == namespace {
			return false
		}
	}
	return true
}
