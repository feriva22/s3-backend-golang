package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	aliyuncredentials "github.com/aliyun/credentials-go/credentials"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
)

var (
	s3Client   *s3.Client
	bucketName string
)

const (
	EnvRoleArn         = "ALIBABA_CLOUD_ROLE_ARN"
	EnvOidcProviderArn = "ALIBABA_CLOUD_OIDC_PROVIDER_ARN"
	EnvOidcTokenFile   = "ALIBABA_CLOUD_OIDC_TOKEN_FILE"
)

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	})
	if err != nil {
		http.Error(w, "Failed to list objects", http.StatusInternalServerError)
		log.Println("Error listing objects:", err)
		return
	}

	files := []string{}
	for _, item := range resp.Contents {
		files = append(files, *item.Key)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func describeFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fileName := "posindonesia.png" // Use example file name or get from request

	//resp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
	//	Bucket: &bucketName,
	//})

	resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    aws.String(fileName),
	})

	if err != nil {
		http.Error(w, "Failed to get object", http.StatusInternalServerError)
		log.Println("Error get object:", err)
		return
	}

	files := []string{}
	files = append(files, *resp.ETag)
	files = append(files, resp.LastModified.String())
	files = append(files, fmt.Sprintf("%d", resp.ContentLength))
	files = append(files, *resp.ContentType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func newOidcCredential() (aliyuncredentials.Credential, error) {
	// https://www.alibabacloud.com/help/doc-detail/378661.html
	aliyunconfig := new(aliyuncredentials.Config).
		SetType("oidc_role_arn").
		SetRoleArn(os.Getenv(EnvRoleArn)).
		SetOIDCProviderArn(os.Getenv(EnvOidcProviderArn)).
		SetOIDCTokenFilePath(os.Getenv(EnvOidcTokenFile)).
		SetRoleSessionName("test-rrsa-oidc-token").
		SetRoleSessionExpiration(3600).    // We set the expiration time of the role session to 3600 seconds (1 hour).
		SetSTSEndpoint("sts-vpc.ap-southeast-5.aliyuncs.com") // https://next.api.aliyun.com/product/Sts

	fmt.Println("Executing this credential using OIDC")
	oidcCredential, err := aliyuncredentials.NewCredential(aliyunconfig)

	return oidcCredential, err
}

type Credentials struct {
	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
}

func (credentials *Credentials) GetAccessKeyID() string {
	return credentials.AccessKeyId
}

func (credentials *Credentials) GetAccessKeySecret() string {
	return credentials.AccessKeySecret
}

func (credentials *Credentials) GetSecurityToken() string {
	return credentials.SecurityToken
}

func NewEnvironmentVariableCredentials() (*Credentials, error) {
	var envCredential *Credentials

	oidcCredential, err := newOidcCredential()
	if err != nil {
		panic(err)
	}
	credentialAliyun, err := oidcCredential.GetCredential()

	accessID := *credentialAliyun.AccessKeyId
	if accessID == "" {
		return envCredential, fmt.Errorf("access key id is empty!")
	}
	accessKey := *credentialAliyun.AccessKeySecret
	if accessKey == "" {
		return envCredential, fmt.Errorf("access key secret is empty!")
	}
	token := *credentialAliyun.SecurityToken
	envCredential = &Credentials{
		AccessKeyId:     accessID,
		AccessKeySecret: accessKey,
		SecurityToken:   token,
	}

	return envCredential, nil
}

func NewAwsS3Provider(credential *Credentials) credentials.StaticCredentialsProvider {

	return credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     credential.AccessKeyId,
			SecretAccessKey: credential.AccessKeySecret,
			SessionToken:    credential.SecurityToken,
		},
	}
}

func main() {
	// Initialize credentials and OSS Comptatible S3 client
	region := os.Getenv("OSS_REGION")
	endpoint := fmt.Sprintf("https://oss-%s.aliyuncs.com", region)

	// Define a custom endpoint resolver
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "oss",
			URL:           endpoint,
			SigningRegion: "us-east-1",
		}, nil
	})

	// Fetch credentials from environment variables
	envCredential, err := NewEnvironmentVariableCredentials()
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	// Provider to using OIDC token
	provider := NewAwsS3Provider(envCredential)

	// Load AWS config from env variables or shared config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(provider),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		log.Fatal("Unable to load AWS SDK config:", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	bucketName = os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("BUCKET_NAME environment variable is required")
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/files", listFilesHandler).Methods("GET")
	r.HandleFunc("/api/files/describe", describeFileHandler).Methods("GET")

	port := "4000"
	log.Println("Server running on http://localhost:" + port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

