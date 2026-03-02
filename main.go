package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
)

var (
	s3Client   *s3.Client
	bucketName string
	presigner  *s3.PresignClient
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

func presignHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key param", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}, func(po *s3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})

	if err != nil {
		log.Println("presign error:", err)
		http.Error(w, "failed to generate presigned url", http.StatusInternalServerError)
		return
	}

	decoded := strings.ReplaceAll(req.URL, `\u0026`, "&")

	log.Println("Presigned URL:", decoded)

	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(map[string]string{
		"url": decoded,
	})
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// target object path in bucket
	objectKey := "uploads/hello.txt" // <-- specific location in bucket

	content := "Hello from Golang uploader!"

	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        strings.NewReader(content),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		log.Println("Upload error:", err)
		http.Error(w, "failed to upload file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "upload success",
		"key":     objectKey,
	})
}

func main() {
	//ossEndpoint := os.Getenv("OSS_ENDPOINT") // e.g. https://oss-ap-southeast-1.aliyuncs.com
	//accessKey := os.Getenv("OSS_ACCESS_KEY")
	//secretKey := os.Getenv("OSS_SECRET_KEY")
	//bucketName = os.Getenv("BUCKET_NAME")
	ossEndpoint := "https://stg-glid-media.posindonesia.co.id"
	accessKey := ""
	secretKey := ""
	bucketName = ""

	if ossEndpoint == "" || accessKey == "" || secretKey == "" {
		log.Fatal("OSS_ENDPOINT, OSS_ACCESS_KEY, OSS_SECRET_KEY, and BUCKET_NAME must be set")
	}

	cfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		),
	}

	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(ossEndpoint)
		o.UsePathStyle = false // IMPORTANT: use virtual-hosted style for OSS
	})

	presigner = s3.NewPresignClient(s3Client)

	r := mux.NewRouter()
	r.HandleFunc("/api/files", listFilesHandler).Methods("GET")
	r.HandleFunc("/api/presign", presignHandler).Methods("GET")
	r.HandleFunc("/api/upload", uploadFileHandler).Methods("POST")
	port := "4000"
	log.Println("Server running on http://localhost:" + port)
	if err := http.ListenAndServe("127.0.0.1:"+port, r); err != nil {
		log.Fatal(err)
	}
}
