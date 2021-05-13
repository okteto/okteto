package diverts

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IngressDivertSpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Value     string `json:"value,omitempty"`
}

type ServiceDivertSpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int    `json:"port"`
}

type DeploymentDivertSpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DivertSpec struct {
	Ingress     IngressDivertSpec    `json:"ingress"`
	FromService ServiceDivertSpec    `json:"fromService"`
	ToService   ServiceDivertSpec    `json:"toService"`
	Deployment  DeploymentDivertSpec `json:"deployment"`
}

type DivertStatus struct {
	OriginalPort       int    `json:"originalPort"`
	ProxyListenerPort  int    `json:"proxyListenerPort"`
	HeaderID           string `json:"headerID"`
	NodeID             string `json:"nodeID"`
	DestinationAddress string `json:"destinationAddress"`
	DestinationPort    int    `json:"destinationPort"`
}

type Divert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DivertSpec   `json:"spec"`
	Status DivertStatus `json:"status"`
}

type DivertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Divert `json:"items"`
}

// DeepCopyInto a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Divert) DeepCopyInto(out *Divert) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy a deepcopy function, copying the receiver, creating a new Divert.
func (in *Divert) DeepCopy() *Divert {
	if in == nil {
		return nil
	}
	out := new(Divert)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject a deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Divert) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DivertList) DeepCopyInto(out *DivertList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Divert, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy a deepcopy function, copying the receiver, creating a new DivertList.
func (in *DivertList) DeepCopy() *DivertList {
	if in == nil {
		return nil
	}
	out := new(DivertList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject a deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DivertList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DivertSpec) DeepCopyInto(out *DivertSpec) {
	*out = *in
}

// DeepCopy a deepcopy function, copying the receiver, creating a new DivertSpec.
func (in *DivertSpec) DeepCopy() *DivertSpec {
	if in == nil {
		return nil
	}
	out := new(DivertSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DivertStatus) DeepCopyInto(out *DivertStatus) {
	*out = *in
}

// DeepCopy a deepcopy function, copying the receiver, creating a new DivertStatus.
func (in *DivertStatus) DeepCopy() *DivertStatus {
	if in == nil {
		return nil
	}
	out := new(DivertStatus)
	in.DeepCopyInto(out)
	return out
}
