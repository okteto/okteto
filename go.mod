module github.com/okteto/okteto

go 1.16

require (
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/a8m/envsubst v1.2.0
	github.com/alessio/shellescape v1.4.1
	github.com/briandowns/spinner v1.16.0
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/containerd/console v1.0.3
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/cli v20.10.8+incompatible
	github.com/docker/docker v20.10.8+incompatible
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dukex/mixpanel v0.0.0-20180925151559-f8d5594f958e
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/fatih/color v1.12.0
	github.com/frankban/quicktest v1.7.3 // indirect
	github.com/gliderlabs/ssh v0.3.3
	github.com/go-git/go-git/v5 v5.4.2
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-getter v1.5.0
	github.com/heroku/docker-registry-client v0.0.0-20190909225348-afc9e1acc3d5
	github.com/joho/godotenv v1.3.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/machinebox/graphql v0.2.2
	github.com/manifoldco/promptui v0.8.0
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/mitchellh/go-ps v1.0.0
	github.com/moby/buildkit v0.8.2
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v3.21.7+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.2.1
	github.com/src-d/enry/v2 v2.1.0
	github.com/subosito/gotenv v1.2.0
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/vbauerster/mpb/v7 v7.0.2
	github.com/whilp/git-urls v1.0.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
	google.golang.org/grpc v1.40.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.4.0
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
