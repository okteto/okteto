package build

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/okteto/okteto/pkg/cache"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model/utils"
	"github.com/spf13/afero"
)

// BuildInfo represents the build info to generate an image
type BuildInfo struct {
	Secrets          BuildSecrets      `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             BuildArgs         `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        BuildDependsOn    `yaml:"depends_on,omitempty"`
}

type VolumeMounts struct {
	LocalPath  string
	RemotePath string
}

// BuildArg is an argument used on the build step.
type BuildArg struct {
	Name  string
	Value string
}

func (v *BuildArg) String() string {
	value, err := env.ExpandEnv(v.Value)
	if err != nil {
		return fmt.Sprintf("%s=%s", v.Name, v.Value)
	}
	return fmt.Sprintf("%s=%s", v.Name, value)
}

// BuildArgs is a list of arguments used on the build step.
type BuildArgs []BuildArg

// BuildDependsOn represents the images that needs to be built before
type BuildDependsOn []string

// BuildSecrets represents the secrets to be injected to the build of the image
type BuildSecrets map[string]string

// ManifestBuild defines all the build section
type ManifestBuild map[string]*BuildInfo

// GetDockerfilePath returns the path to the Dockerfile
func (b *BuildInfo) GetDockerfilePath() string {
	if filepath.IsAbs(b.Dockerfile) {
		return b.Dockerfile
	}
	fs := afero.NewOsFs()
	joinPath := filepath.Join(b.Context, b.Dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath, fs) {
		oktetoLog.Infof("Dockerfile '%s' is not in a relative path to context '%s'", b.Dockerfile, b.Context)
		return b.Dockerfile
	}

	if joinPath != filepath.Clean(b.Dockerfile) && filesystem.FileExistsAndNotDir(b.Dockerfile, fs) {
		oktetoLog.Infof("Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'", b.Context, b.Dockerfile)
	}

	return joinPath
}

// AddBuildArgs add a set of args to the build information
func (b *BuildInfo) AddBuildArgs(previousImageArgs map[string]string) error {
	if err := b.expandManifestBuildArgs(previousImageArgs); err != nil {
		return err
	}
	return b.addExpandedPreviousImageArgs(previousImageArgs)
}

func (b *BuildInfo) expandManifestBuildArgs(previousImageArgs map[string]string) (err error) {
	for idx, arg := range b.Args {
		if val, ok := previousImageArgs[arg.Name]; ok {
			oktetoLog.Infof("overriding '%s' with the content of previous build", arg.Name)
			arg.Value = val
		}
		arg.Value, err = env.ExpandEnv(arg.Value)
		if err != nil {
			return err
		}
		b.Args[idx] = arg
	}
	return nil
}

func (b *BuildInfo) addExpandedPreviousImageArgs(previousImageArgs map[string]string) error {
	alreadyAddedArg := map[string]bool{}
	for _, arg := range b.Args {
		alreadyAddedArg[arg.Name] = true
	}
	for k, v := range previousImageArgs {
		if _, ok := alreadyAddedArg[k]; ok {
			continue
		}
		expandedValue, err := env.ExpandEnv(v)
		if err != nil {
			return err
		}
		b.Args = append(b.Args, BuildArg{
			Name:  k,
			Value: expandedValue,
		})
		oktetoLog.Infof("Added '%s' to build args", k)
	}
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (e *BuildArg) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}
	maxBuildArgsParts := 2

	parts := strings.SplitN(raw, "=", maxBuildArgsParts)
	e.Name = parts[0]
	if len(parts) == maxBuildArgsParts {
		e.Value = parts[1]
		return nil
	}

	e.Name, err = env.ExpandEnv(parts[0])
	if err != nil {
		return err
	}
	e.Value = parts[0]
	return nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (d *BuildDependsOn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		*d = BuildDependsOn{rawString}
		return nil
	}

	var rawStringList []string
	err = unmarshal(&rawStringList)
	if err == nil {
		*d = rawStringList
		return nil
	}
	return err
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawString string
	err := unmarshal(&rawString)
	if err == nil {
		buildInfo.Name = rawString
		return nil
	}

	var rawBuildInfo buildInfoRaw
	err = unmarshal(&rawBuildInfo)
	if err != nil {
		return err
	}

	buildInfo.Name = rawBuildInfo.Name
	buildInfo.Context = rawBuildInfo.Context
	buildInfo.Dockerfile = rawBuildInfo.Dockerfile
	buildInfo.Target = rawBuildInfo.Target
	buildInfo.Args = rawBuildInfo.Args
	buildInfo.Image = rawBuildInfo.Image
	buildInfo.CacheFrom = rawBuildInfo.CacheFrom
	buildInfo.ExportCache = rawBuildInfo.ExportCache
	buildInfo.DependsOn = rawBuildInfo.DependsOn
	buildInfo.Secrets = rawBuildInfo.Secrets
	return nil
}

// BuildInfoRaw represents the build info for serialization
type buildInfoRaw struct {
	Secrets          BuildSecrets      `yaml:"secrets,omitempty"`
	Name             string            `yaml:"name,omitempty"`
	Context          string            `yaml:"context,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	Target           string            `yaml:"target,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	CacheFrom        cache.CacheFrom   `yaml:"cache_from,omitempty"`
	Args             BuildArgs         `yaml:"args,omitempty"`
	VolumesToInclude []VolumeMounts    `yaml:"-"`
	ExportCache      cache.ExportCache `yaml:"export_cache,omitempty"`
	DependsOn        BuildDependsOn    `yaml:"depends_on,omitempty"`
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (buildInfo *BuildInfo) MarshalYAML() (interface{}, error) {
	if buildInfo.Context != "" && buildInfo.Context != "." {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Dockerfile != "" && buildInfo.Dockerfile != "./Dockerfile" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Target != "" {
		return buildInfoRaw(*buildInfo), nil
	}
	if buildInfo.Args != nil && len(buildInfo.Args) != 0 {
		return buildInfoRaw(*buildInfo), nil
	}
	return buildInfo.Name, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (ba *BuildArgs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	buildArgs := make(BuildArgs, 0)
	result, err := getBuildArgs(unmarshal)
	if err != nil {
		return err
	}
	for key, value := range result {
		buildArgs = append(buildArgs, BuildArg{Name: key, Value: value})
	}
	sort.SliceStable(buildArgs, func(i, j int) bool {
		return strings.Compare(buildArgs[i].Name, buildArgs[j].Name) < 0
	})
	*ba = buildArgs
	return nil
}

// MarshalYAML Implements the marshaler interface of the yaml pkg.
func (v VolumeMounts) MarshalYAML() (interface{}, error) {
	return v.RemotePath, nil
}

// UnmarshalYAML Implements the Unmarshaler interface of the yaml pkg.
func (v *VolumeMounts) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	stackVolumePartsOnlyRemote := 1
	stackVolumeParts := 2
	stackVolumeMaxParts := 3

	parts := strings.Split(raw, ":")
	if runtime.GOOS == "windows" {
		if len(parts) >= stackVolumeMaxParts {
			localPath := fmt.Sprintf("%s:%s", parts[0], parts[1])
			if filepath.IsAbs(localPath) {
				parts = append([]string{localPath}, parts[2:]...)
			}
		}
	}

	if len(parts) == stackVolumeParts {
		v.LocalPath = parts[0]
		v.RemotePath = parts[1]
	} else if len(parts) == stackVolumePartsOnlyRemote {
		v.RemotePath = parts[0]
	} else {
		return fmt.Errorf("Syntax error volumes should be 'local_path:remote_path' or 'remote_path'")
	}

	return nil
}

// ToString returns volume as string
func (v VolumeMounts) ToString() string {
	if v.LocalPath != "" {
		return fmt.Sprintf("%s:%s", v.LocalPath, v.RemotePath)
	}
	return v.RemotePath
}

func getBuildArgs(unmarshal func(interface{}) error) (map[string]string, error) {
	result := make(map[string]string)

	var rawList []BuildArg
	err := unmarshal(&rawList)
	if err == nil {
		for _, buildArg := range rawList {
			value, err := env.ExpandEnvIfNotEmpty(buildArg.Value)
			if err != nil {
				return nil, err
			}
			result[buildArg.Name] = value
		}
		return result, nil
	}
	var rawMap map[string]string
	err = unmarshal(&rawMap)
	if err != nil {
		return nil, err
	}
	for key, value := range rawMap {
		result[key], err = env.ExpandEnvIfNotEmpty(value)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Copy clones the buildInfo without the pointers
func (b *BuildInfo) Copy() *BuildInfo {
	result := &BuildInfo{
		Name:        b.Name,
		Context:     b.Context,
		Dockerfile:  b.Dockerfile,
		Target:      b.Target,
		Image:       b.Image,
		ExportCache: b.ExportCache,
	}

	// copy to new pointers
	cacheFrom := []string{}
	cacheFrom = append(cacheFrom, b.CacheFrom...)
	result.CacheFrom = cacheFrom

	args := BuildArgs{}
	args = append(args, b.Args...)
	result.Args = args

	secrets := BuildSecrets{}
	for k, v := range b.Secrets {
		secrets[k] = v
	}
	result.Secrets = secrets

	volumesToMount := []VolumeMounts{}
	volumesToMount = append(volumesToMount, b.VolumesToInclude...)
	result.VolumesToInclude = volumesToMount

	dependsOn := BuildDependsOn{}
	dependsOn = append(dependsOn, b.DependsOn...)
	result.DependsOn = dependsOn

	return result
}

func (b *BuildInfo) SetBuildDefaults() {
	if b.Context == "" {
		b.Context = "."
	}

	if _, err := url.ParseRequestURI(b.Context); err != nil && b.Dockerfile == "" {
		b.Dockerfile = "Dockerfile"
	}

}

func (b *ManifestBuild) Validate() error {
	cycle := utils.GetDependentCyclic(b.toGraph())
	if len(cycle) == 1 { // depends on the same node
		return fmt.Errorf("manifest build validation failed: image '%s' is referenced on its dependencies", cycle[0])
	} else if len(cycle) > 1 {
		svcsDependents := fmt.Sprintf("%s and %s", strings.Join(cycle[:len(cycle)-1], ", "), cycle[len(cycle)-1])
		return fmt.Errorf("manifest validation failed: cyclic dependendecy found between %s", svcsDependents)
	}
	return nil
}

// GetSvcsToBuildFromList returns the builds from a list and all its
func (b *ManifestBuild) GetSvcsToBuildFromList(toBuild []string) []string {
	initialSvcsToBuild := toBuild
	svcsToBuildWithDependencies := utils.GetDependentNodes(b.toGraph(), toBuild)
	if len(initialSvcsToBuild) != len(svcsToBuildWithDependencies) {
		dependantBuildImages := utils.GetListDiff(initialSvcsToBuild, svcsToBuildWithDependencies)
		oktetoLog.Warning("The following build images need to be built because of dependencies: [%s]", strings.Join(dependantBuildImages, ", "))
	}
	return svcsToBuildWithDependencies
}

func (b ManifestBuild) toGraph() utils.Graph {
	g := utils.Graph{}
	for k, v := range b {
		g[k] = v.DependsOn
	}
	return g
}
