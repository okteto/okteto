module github.com/okteto/okteto

require (
	cloud.google.com/go v0.39.0 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver v1.4.2
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/briandowns/spinner v1.7.0
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cheggaaa/pb/v3 v3.0.1
	github.com/containerd/console v0.0.0-20181022165439-0650fd9eeb50
	github.com/containerd/fifo v0.0.0-20190816180239-bda0ff6ed73c // indirect
	github.com/containerd/ttrpc v0.0.0-20191025122922-cf7f4d5f2d61 // indirect
	github.com/containerd/typeurl v0.0.0-20190911142611-5eb25027c9fd // indirect
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v0.0.0-20200130152716-5d0cf8839492
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/fatih/color v1.7.0
	github.com/gofrs/flock v0.7.1
	github.com/gogo/googleapis v1.3.0 // indirect
	github.com/google/go-github/v28 v28.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/hashicorp/go-getter v1.0.2
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/lib/pq v1.3.0 // indirect
	github.com/machinebox/graphql v0.2.2
	github.com/manifoldco/promptui v0.3.2
	github.com/matryer/is v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/mattn/psutil v0.0.0-20170126005127-e6c88f1e9be6
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/moby/buildkit v0.6.2
	github.com/nicksnyder/go-i18n v1.10.0 // indirect
	github.com/opencontainers/runtime-spec v1.0.1 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rubenv/sql-migrate v0.0.0-20200212082348-64f95ea68aa3 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/skratchdot/open-golang v0.0.0-20190402232053-79abb63cd66e
	github.com/spf13/cobra v0.0.5
	github.com/src-d/enry/v2 v2.1.0
	go.undefinedlabs.com/scopeagent v0.0.0-20200123164547-e307ed3a9216
	golang.org/x/crypto v0.0.0-20200128174031-69ecbb4d6d5d
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	google.golang.org/genproto v0.0.0-20191009194640-548a555dbc03 // indirect
	google.golang.org/grpc v1.27.0
	gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20180810215634-df19058c872c // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	gopkg.in/yaml.v2 v2.2.5
	helm.sh/helm v2.16.3+incompatible
	helm.sh/helm/v3 v3.1.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/cli-runtime v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/helm v2.16.3+incompatible
	k8s.io/kubectl v0.17.2
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/containerd/containerd v1.3.0-0.20190507210959-7c1e88399ec0 => github.com/containerd/containerd v1.3.0
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
	github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	golang.org/x/crypto v0.0.0-20190129210102-0709b304e793 => golang.org/x/crypto v0.0.0-20180904163835-0709b304e793
)

go 1.13
