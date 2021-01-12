module github.com/okteto/okteto

go 1.16

require (
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver v1.5.0
	github.com/a8m/envsubst v1.2.0
	github.com/alessio/shellescape v1.3.0
	github.com/briandowns/spinner v1.11.1
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/containerd/console v1.0.0
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v0.0.0-20200227165822-2298e6a3fe24
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/fatih/color v1.9.0
	github.com/frankban/quicktest v1.7.3 // indirect
	github.com/gliderlabs/ssh v0.3.1
	github.com/go-git/go-git/v5 v5.1.0
	github.com/gofrs/flock v0.7.1
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/hashicorp/go-getter v1.5.0
	github.com/heroku/docker-registry-client v0.0.0-20190909225348-afc9e1acc3d5
	github.com/kr/pretty v0.2.0 // indirect
	github.com/machinebox/graphql v0.2.2
	github.com/manifoldco/promptui v0.3.2
	github.com/matryer/is v1.2.0 // indirect
	github.com/mattn/psutil v0.0.0-20170126005127-e6c88f1e9be6
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/mitchellh/go-ps v1.0.0
	github.com/moby/buildkit v0.7.2-1
	github.com/nicksnyder/go-i18n v1.10.0 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/skratchdot/open-golang v0.0.0-20190402232053-79abb63cd66e
	github.com/spf13/cobra v1.1.1
	github.com/src-d/enry/v2 v2.1.0
	github.com/subosito/gotenv v1.2.0
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6 // indirect
	google.golang.org/grpc v1.28.0
	google.golang.org/protobuf v1.24.0 // indirect
	gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20180810215634-df19058c872c // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.3.0
	helm.sh/helm/v3 v3.3.4
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/cli-runtime v0.18.8
	k8s.io/client-go v0.18.8
	k8s.io/kubectl v0.18.8
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.9

replace github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200227233006-38f52c9fec82

replace github.com/moby/buildkit => github.com/okteto/buildkit v0.7.2-1
