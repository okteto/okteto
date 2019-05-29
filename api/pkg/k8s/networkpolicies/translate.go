package networkpolicies

import (
	"os"

	"github.com/okteto/app/api/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func translate(s *model.Space) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.GetNetworkPolicyName(),
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress},
			Ingress: []netv1.NetworkPolicyIngressRule{
				netv1.NetworkPolicyIngressRule{
					From: []netv1.NetworkPolicyPeer{
						netv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									metav1.LabelSelectorRequirement{
										Key:      "app.kubernetes.io/name",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{s.ID, "ingress-nginx"},
									},
								},
							},
						},
					},
				},
			},
			Egress: []netv1.NetworkPolicyEgressRule{
				netv1.NetworkPolicyEgressRule{
					To: []netv1.NetworkPolicyPeer{
						netv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app.kubernetes.io/name": s.ID},
							},
						},
						netv1.NetworkPolicyPeer{
							IPBlock: &netv1.IPBlock{
								CIDR:   "0.0.0.0/0",
								Except: []string{os.Getenv("OKTETO_CLUSTER_CIDR"), "169.254.169.254/32"},
							},
						},
					},
				},
			},
		},
	}
}

func translateDNS(s *model.Space) *netv1.NetworkPolicy {
	protocol := apiv1.ProtocolUDP
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.GetDNSPolicyName(),
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeEgress},
			Egress: []netv1.NetworkPolicyEgressRule{
				netv1.NetworkPolicyEgressRule{
					To: []netv1.NetworkPolicyPeer{
						netv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app.kubernetes.io/name": "kube-system"},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"k8s-app": "kube-dns"},
							},
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						netv1.NetworkPolicyPort{
							Port: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: int32(53),
							},
							Protocol: &protocol,
						},
					},
				},
			},
		},
	}
}
