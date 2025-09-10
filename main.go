package main

import (
        "context"
        "encoding/json"
        "log"
        "net/http"
        "os"
        "fmt"

        "github.com/aws/aws-sdk-go-v2/aws"
        "github.com/aws/aws-sdk-go-v2/config"
        "github.com/aws/aws-sdk-go-v2/service/s3"
        "github.com/gorilla/mux"
)

var (
        s3Client   *s3.Client
        bucketName string
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

func main() {
        // Load AWS config from env variables or shared config
        endpoint = os.Getenv("OSS_ENDPOINT")
        if endpoint == "" {
                log.Fatal("OSS_ENDPOINT environment variable is required")
        }

        // Define a custom endpoint resolver
        customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
                return aws.Endpoint{
                        PartitionID:   "oss",
                        URL:           endpoint,
                        SigningRegion: "us-east-1",
                }, nil
        })

        cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"),config.WithEndpointResolverWithOptions(customResolver),)
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

        port := "4000"
        log.Println("Server running on http://localhost:" + port)
        if err := http.ListenAndServe(":"+port, r); err != nil {
                log.Fatal(err)
        }
}
