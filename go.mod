module github.com/okteto/okteto

go 1.18

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/a8m/envsubst v1.3.0
	github.com/alessio/shellescape v1.4.1
	github.com/briandowns/spinner v1.23.0
	github.com/cheggaaa/pb/v3 v3.1.0
	github.com/chzyer/readline v1.5.1
	github.com/compose-spec/godotenv v1.1.1
	github.com/containerd/console v1.0.3
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v20.10.12+incompatible
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/fatih/color v1.13.0
	github.com/gliderlabs/ssh v0.3.5
	github.com/go-git/go-git/v5 v5.4.2
	github.com/google/go-containerregistry v0.8.0 // when updating need google.golang.org/grpc 1.29
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-getter v1.6.2
	github.com/juju/ansiterm v1.0.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/manifoldco/promptui v0.9.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/moby/buildkit v0.9.2
	github.com/moby/term v0.0.0-20220808134915-39b0c02b01ae
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/shurcooL/graphql v0.0.0-20220606043923-3cf50f8a0a29
	github.com/sirupsen/logrus v1.9.0
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/afero v1.9.2
	github.com/spf13/cobra v1.5.0
	github.com/src-d/enry/v2 v2.1.0
	github.com/stern/stern v1.22.0
	github.com/vbauerster/mpb/v7 v7.5.3
	github.com/whilp/git-urls v1.0.0
	golang.org/x/crypto v0.0.0-20220924013350-4ba4fb4dd9e7
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1
	golang.org/x/sync v0.0.0-20220907140024-f12130a52804
	golang.org/x/term v0.5.0
	google.golang.org/grpc v1.47.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.25.2
	k8s.io/apimachinery v0.25.2
	k8s.io/cli-runtime v0.25.2
	k8s.io/client-go v0.25.2
	k8s.io/kubectl v0.25.2
	k8s.io/utils v0.0.0-20220922133306-665eaaec4324
)

require (
	cloud.google.com/go v0.99.0 // indirect
	cloud.google.com/go/storage v1.14.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Microsoft/hcsshim v0.8.25 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210428141323-04723f9f07d7 // indirect
	github.com/Sirupsen/logrus v0.0.0-00010101000000-000000000000 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/andybalholm/brotli v1.0.1 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/aws/aws-sdk-go v1.44.203 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/containerd v1.5.18 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/creack/pty v1.1.17 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go v1.5.1-1.0.20160303222718-d30aec9fd63c // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/docker/libnetwork v0.5.6 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gofrs/flock v0.8.0 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4-0.20210608040537-544b4180ac70 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mholt/archiver/v3 v3.5.1
	github.com/miekg/pkcs11 v1.0.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.5 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.12.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/src-d/go-oniguruma v1.1.0 // indirect
	github.com/stretchr/testify v1.8.0
	github.com/theupdateframework/notary v0.7.0 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20210609172227-d72af97c0eaf // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/tonistiigi/vt100 v0.0.0-20210615222946-8066bb97264f // indirect
	github.com/toqueteos/trie v1.0.0 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	github.com/withfig/autocomplete-tools/packages/cobra v0.0.0-20211118163844-94616c903bcb
	github.com/xanzy/ssh-agent v0.3.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/contrib v0.21.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.21.0 // indirect
	go.opentelemetry.io/otel v1.0.0-RC1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.0.0-RC1 // indirect
	go.opentelemetry.io/otel/sdk v1.0.0-RC1 // indirect
	go.opentelemetry.io/otel/trace v1.0.0-RC1 // indirect
	go.opentelemetry.io/proto/otlp v0.9.0 // indirect
	go.starlark.net v0.0.0-20220817180228-f738f5508c12 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.0.0-20220922220347-f3bd1da661af // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.62.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220628213854-d9e0b6570c03 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/toqueteos/substring.v1 v1.0.2 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/component-base v0.25.2 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220803164354-a70c9af30aea // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

require (
	istio.io/api v0.0.0-20221013011440-bc935762d2b9
	istio.io/client-go v1.15.3
)

replace (
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.8.0
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	github.com/moby/buildkit => github.com/okteto/buildkit v0.9.2-okteto2

	// https://github.com/okteto/okteto/issues/2129
	google.golang.org/grpc => google.golang.org/grpc v1.40.0
)
