package networkpolicies

import (
	"github.com/okteto/app/api/model"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(s *model.Space) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{
				netv1.NetworkPolicyIngressRule{
					From: []netv1.NetworkPolicyPeer{
						netv1.NetworkPolicyPeer{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"name": s.ID},
							},
						},
					},
				},
			},
			// TODO block cloud init and 10255 (public kubelet api)
			// Egress: []netv1.NetworkPolicyEgressRule{
			// 	netv1.NetworkPolicyEgressRule{
			// 		To: []netv1.NetworkPolicyPeer{
			// 			netv1.NetworkPolicyPeer{
			// 				NamespaceSelector: &metav1.LabelSelector{
			// 					MatchLabels: map[string]string{"name": s.ID},
			// 				},
			// 			},
			// 		},
			// 	},
			// },
		},
	}
}
