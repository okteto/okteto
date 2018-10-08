package fetcher

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

//S3 uses authenticated HEAD requests to poll the status of a given
//object. If it detects this file has been updated, it will perform
//an object GET and return its io.Reader stream.
type S3 struct {
	Access string
	Secret string
	Region string
	Bucket string
	Key    string
	//interal state
	Interval time.Duration
	client   *s3.S3
	delay    bool
	lastETag string
}

// Init validates the provided config
func (s *S3) Init() error {
	if s.Bucket == "" {
		return errors.New("S3 bucket not set")
	} else if s.Key == "" {
		return errors.New("S3 key not set")
	}
	if s.Region == "" {
		s.Region = "ap-southeast-2"
	}
	creds := credentials.AnonymousCredentials
	if s.Access != "" {
		creds = credentials.NewStaticCredentials(s.Access, s.Secret, "")
	} else if os.Getenv("AWS_ACCESS_KEY") != "" {
		creds = credentials.NewEnvCredentials()
	}
	config := &aws.Config{
		Credentials: creds,
		Region:      &s.Region,
	}
	s.client = s3.New(session.New(config))
	//apply defaults
	if s.Interval == 0 {
		s.Interval = 5 * time.Minute
	}
	return nil
}

// Fetch the binary from S3
func (s *S3) Fetch() (io.Reader, error) {
	//delay fetches after first
	if s.delay {
		time.Sleep(s.Interval)
	}
	s.delay = true
	//status check using HEAD
	head, err := s.client.HeadObject(&s3.HeadObjectInput{Bucket: &s.Bucket, Key: &s.Key})
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed (%s)", err)
	}
	if s.lastETag == *head.ETag {
		return nil, nil //skip, file match
	}
	s.lastETag = *head.ETag
	//binary fetch using GET
	get, err := s.client.GetObject(&s3.GetObjectInput{Bucket: &s.Bucket, Key: &s.Key})
	if err != nil {
		return nil, fmt.Errorf("GET request failed (%s)", err)
	}
	//extract gz files
	if strings.HasSuffix(s.Key, ".gz") && aws.StringValue(get.ContentEncoding) != "gzip" {
		return gzip.NewReader(get.Body)
	}
	//success!
	return get.Body, nil
}
