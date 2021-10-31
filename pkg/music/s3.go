package music

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3 struct {
	client *s3.S3
	bucket string
}

func NewS3(endpoint, region, bucket, access, secret string) (*S3, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Credentials:      credentials.NewStaticCredentials(access, secret, ""),
			Endpoint:         aws.String(endpoint),
			Region:           aws.String(region),
			S3ForcePathStyle: aws.Bool(true),
		},
		Profile: "s3",
	})

	// check if the session was created correctly.
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	s3Client := s3.New(sess)

	return &S3{
		client: s3Client,
		bucket: bucket,
	}, nil
}

func (s *S3) Get(file string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(file),
	}

	result, err := s.client.GetObject(input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}

	return result.Body, nil
}

func (s *S3) Put(file string, data io.ReadSeeker) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(file),
		Body:   data,
	}

	_, err := s.client.PutObject(input)
	if err != nil {
		return fmt.Errorf("failed to put object: %v", err)
	}

	return nil
}
