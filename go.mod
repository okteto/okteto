module github.com/okteto/okteto

go 1.16

require (
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/a8m/envsubst v1.2.0
	github.com/alessio/shellescape v1.3.0
	github.com/briandowns/spinner v1.11.1
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/containerd/console v1.0.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v20.10.0-beta1.0.20201029214301-1d20b15adc38+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/fatih/color v1.9.0
	github.com/frankban/quicktest v1.7.3 // indirect
	github.com/gliderlabs/ssh v0.3.1
	github.com/go-git/go-git/v5 v5.1.0
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-getter v1.5.0
	github.com/heroku/docker-registry-client v0.0.0-20190909225348-afc9e1acc3d5
	github.com/joho/godotenv v1.3.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/machinebox/graphql v0.2.2
	github.com/manifoldco/promptui v0.3.2
	github.com/matryer/is v1.2.0 // indirect
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/mitchellh/go-ps v1.0.0
	github.com/moby/buildkit v0.8.2
	github.com/moby/term v0.0.0-20200915141129-7f0af18e79f2
	github.com/nicksnyder/go-i18n v1.10.0 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v3.21.1+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/skratchdot/open-golang v0.0.0-20190402232053-79abb63cd66e
	github.com/spf13/cobra v1.1.1
	github.com/src-d/enry/v2 v2.1.0
	github.com/subosito/gotenv v1.2.0
	github.com/vbauerster/mpb/v7 v7.0.2
	github.com/whilp/git-urls v1.0.0
	golang.org/x/crypto v0.0.0-20201216223049-8b5274cf687f
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/term v0.0.0-20201117132131-f5c789dd3221
	google.golang.org/grpc v1.29.1
	gopkg.in/alecthomas/kingpin.v3-unstable v3.0.0-20180810215634-df19058c872c // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.3.0
	helm.sh/helm/v3 v3.5.1
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/cli-runtime v0.20.1
	k8s.io/client-go v0.20.1
	k8s.io/kubectl v0.20.1
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/moby/buildkit => github.com/okteto/buildkit v0.8.3-okteto1
