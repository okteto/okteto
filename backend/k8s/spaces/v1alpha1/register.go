package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//GroupName identifies our CRDs
const GroupName = "crd.okteto.com"

//GroupVersion identifies the api version of our CRDs
const GroupVersion = "v1alpha1"

//SchemeGroupVersion identifies the group schema version of our CRDs
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

var (
	// SchemeBuilder TBD
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme TBD
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Space{},
		&SpaceList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
