module github.com/okteto/okteto

require (
	cloud.google.com/go v0.39.0 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver v1.4.2
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf // indirect
	github.com/briandowns/spinner v1.7.0
	github.com/cespare/trie v0.0.0-20150610204604-3fe1a95cbba9 // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cheggaaa/pb/v3 v3.0.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fatih/color v1.7.0
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-openapi/spec v0.18.0 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-github/v28 v28.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/go-getter v1.0.2
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/machinebox/graphql v0.2.2
	github.com/manifoldco/promptui v0.3.2
	github.com/matryer/is v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/mattn/psutil v0.0.0-20170126005127-e6c88f1e9be6
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/moovweb/rubex v0.0.0-20160224233712-cb849acea614 // indirect
	github.com/nicksnyder/go-i18n v1.10.0 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.1
	github.com/skratchdot/open-golang v0.0.0-20190402232053-79abb63cd66e
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/toqueteos/trie v0.0.0-20150530104557-56fed4a05683 // indirect
	go.undefinedlabs.com/scopeagent v0.0.0-20191016145518-eb0794995bac
	golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422 // indirect
	golang.org/x/net v0.0.0-20191014212845-da9a3fd4c582 // indirect
	golang.org/x/oauth2 v0.0.0-20190523182746-aaccbc9213b0 // indirect
	golang.org/x/sys v0.0.0-20191010194322-b09406accb47 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20180810215634-df19058c872c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/src-d/enry.v1 v1.6.7
	gopkg.in/toqueteos/substring.v1 v1.0.2 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/kubernetes v1.15.2
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190805141119-fdd30b57c827
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190805143126-cdb999c96590
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190805142138-368b2058237c
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190805143448-a07e59fb081d
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190805141520-2fe0317bcee0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190805144409-8484242760e7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190805144246-c01ee70854a1
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190805141645-3a5e5ac800ae
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190531030430-6117653b35f1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190805144531-3985229e1802
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190805142416-fd821fbbb94e
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190805144128-269742da31dd
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190805143734-7f1675b90353
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190805144012-2a1ed1f3d8a4
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20190602132728-7075c07e78bf
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190805143852-517ff267f8d1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190805144654-3d5bf3a310c1
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190805143318-16b07057415d
	k8s.io/node-api => k8s.io/node-api v0.0.0-20190805144819-9dd62e4d5327
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190805142637-3b65bc4bb24f
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.0.0-20190805143616-1485e5142db3
	k8s.io/sample-controller => k8s.io/sample-controller v0.0.0-20190805142825-b16fad786282
)

go 1.13
