package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *Space) DeepCopyInto(out *Space) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
}

// DeepCopyObject returns a generically typed copy of an object
func (in *Space) DeepCopyObject() runtime.Object {
	out := Space{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *SpaceList) DeepCopyObject() runtime.Object {
	out := SpaceList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]Space, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
