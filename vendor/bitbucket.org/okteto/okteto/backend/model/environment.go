package model

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

const (
	//AWS is the amazon web services provider
	AWS = "aws"

	//Demo is the ademo provider
	Demo = "demo"

	//K8 is the kubernetes provider
	K8 = "k8"
)

//Environment represents a environment.yml file
type Environment struct {
	ID          string       `yaml:"id,omitempty"`
	Name        string       `yaml:"name,omitempty"`
	DNSProvider *DNSProvider `yaml:"dns,omitempty"`
	Provider    *Provider    `yaml:"provider,omitempty"`
	Registry    *Registry    `yaml:"registry,omitempty"`
}

//DNSProvider represents the info for the cloud provider where the DNS is created
type DNSProvider struct {
	AccessKey    string `yaml:"access_key,omitempty"`
	SecretKey    string `yaml:"secret_key,omitempty"`
	HostedZone   string `yaml:"hosted_zone,omitempty"`
	HostedZoneID string `yaml:"hosted_zone_id,omitempty"`
}

//Provider represents the info for the cloud provider where the service takes place
type Provider struct {
	Type            string    `yaml:"type,omitempty"`
	Username        string    `yaml:"username,omitempty"`
	Password        string    `yaml:"password,omitempty"`
	Endpoint        string    `yaml:"endpoint,omitempty"`
	CaCert          string    `yaml:"ca_cert,omitempty"`
	AccessKey       string    `yaml:"access_key,omitempty"`
	SecretKey       string    `yaml:"secret_key,omitempty"`
	Region          string    `yaml:"region,omitempty"`
	Ami             string    `yaml:"ami,omitempty"`
	InstanceType    string    `yaml:"instance_type,omitempty"`
	InstanceProfile string    `yaml:"instance_profile,omitempty"`
	Vpc             string    `yaml:"vpc,omitempty"`
	Subnets         []*string `yaml:"subnets,omitempty"`
	SecurityGroup   string    `yaml:"security_group,omitempty"`
	Keypair         string    `yaml:"keypair,omitempty"`
	HostedZone      string    `yaml:"hosted_zone,omitempty"`
	Certificate     string    `yaml:"certificate,omitempty"`
}

//Registry represents Docker Registry credentials
type Registry struct {
	Server   string `yaml:"server,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

//Validate returns an error for invalid environment.yml files
func (e *Environment) Validate() error {
	if e.Provider.GetType() == Demo {
		return nil
	}
	if e.Name == "" {
		return fmt.Errorf("'environment.name' is mandatory")
	}
	if !isAlphaNumeric(e.Name) {
		return fmt.Errorf("'environment.name' only allows alphanumeric characters or dashes")
	}
	if err := e.Provider.Validate(); err != nil {
		return err
	}
	if e.DNSProvider == nil {
		if e.Provider.HostedZone == "" {
			return fmt.Errorf("'environment.provider.hosted_zone' must be defined if no dns provider is defined")
		}
		return nil
	}
	return e.DNSProvider.Validate()
}

//Domain returns the seacrh domain for a given environment
func (e *Environment) Domain() string {
	var domain string
	if e.Provider.HostedZone == "" {
		domain = strings.TrimSuffix(e.DNSProvider.HostedZone, ".")
	} else {
		domain = strings.TrimSuffix(e.Provider.HostedZone, ".")
	}
	return fmt.Sprintf("%s.%s", e.Name, domain)
}

//GetConfig returns a config aws object
func (p *Provider) GetConfig() *aws.Config {
	awsConfig := &aws.Config{
		Region:      aws.String(p.Region),
		Credentials: credentials.NewStaticCredentials(p.AccessKey, p.SecretKey, ""),
	}
	return awsConfig
}

//GetType returns the type of a provider
func (p *Provider) GetType() string {
	return strings.ToLower(p.Type)
}

//Validate returns an error for invalid providers
func (p *Provider) Validate() error {
	switch p.GetType() {
	case "":
		return fmt.Errorf("'provider.type' cannot be empty")
	case Demo:
		return nil
	case AWS:
		if p.AccessKey == "" {
			return fmt.Errorf("'provider.access_key' cannot be empty")
		}
		if p.SecretKey == "" {
			return fmt.Errorf("'provider.secret_key' cannot be empty")
		}
		if p.Region == "" {
			return fmt.Errorf("'provider.region' cannot be empty")
		}
		return nil
	case K8:
		if p.Username == "" {
			return fmt.Errorf("'provider.username' cannot be empty")
		}
		if p.Password == "" {
			return fmt.Errorf("'provider.password' cannot be empty")
		}
		if p.Endpoint == "" {
			return fmt.Errorf("'provider.endpoint' cannot be empty")
		}
		if p.CaCert == "" {
			return fmt.Errorf("'provider.ca_cert' cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("'provider.type' '%s' is not supported", p.GetType())
	}
}

//GetConfig returns a config aws object
func (p *DNSProvider) GetConfig() *aws.Config {
	awsConfig := &aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: credentials.NewStaticCredentials(p.AccessKey, p.SecretKey, ""),
	}
	return awsConfig
}

//Validate returns an error for invalid providers
func (p *DNSProvider) Validate() error {
	if p.AccessKey == "" {
		return fmt.Errorf("'provider.access_key' cannot be empty")
	}
	if p.SecretKey == "" {
		return fmt.Errorf("'provider.secret_key' cannot be empty")
	}
	if p.HostedZone == "" {
		return fmt.Errorf("'provider.hosted_zone' cannot be empty")
	}
	svc := route53.New(session.New(), p.GetConfig())
	hostedZonesInput := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(p.HostedZone),
		MaxItems: aws.String("1"),
	}
	resp, err := svc.ListHostedZonesByName(hostedZonesInput)
	if err != nil {
		return err
	}
	if len(resp.HostedZones) != 1 {
		return fmt.Errorf("Hosted zone '%s' not found", p.HostedZone)
	}
	p.HostedZoneID = *resp.HostedZones[0].Id
	return nil
}

var dockerConfigTemplate = `
{
	"auths": {
		"%s": {
			"auth": "%s"
		}
	}
}
`

//B64DockerConfig conputes the base64 format of docker credentials
func (e *Environment) B64DockerConfig() string {
	if e.Registry == nil || e.Registry.Username == "" || e.Registry.Password == "" {
		return ""
	}
	if e.Registry.Server == "" {
		e.Registry.Server = "https://index.docker.io/v1/"
	}
	auth := fmt.Sprintf("%s:%s", e.Registry.Username, e.Registry.Password)
	authEncoded := base64.StdEncoding.EncodeToString([]byte(auth))
	config := fmt.Sprintf(dockerConfigTemplate, e.Registry.Server, authEncoded)
	return base64.StdEncoding.EncodeToString([]byte(config))
}
