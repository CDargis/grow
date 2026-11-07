package oils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/oklog/ulid/v2"
)

// maxCOABytes caps a fetched COA. Real COAs are a few hundred KB to a few
// MB; the cap keeps a bad link from filling Lambda memory (256 MB).
const maxCOABytes = 25 << 20

// S3Archiver downloads a COA from its public link and stores a copy in the
// media bucket, so the record survives the lab moving or deleting the file.
// Whatever the link returns is stored as-is: usually a PDF, sometimes a lab
// portal's HTML page.
type S3Archiver struct {
	client *http.Client
	s3     *s3.Client
	bucket string
}

func NewS3Archiver(s3Client *s3.Client, bucket string) *S3Archiver {
	// Lambda's timeout is 30s; leave room for the S3 write and the DynamoDB put.
	return &S3Archiver{client: &http.Client{Timeout: 15 * time.Second}, s3: s3Client, bucket: bucket}
}

func (a *S3Archiver) Archive(ctx context.Context, rawURL, keyPrefix string) (string, error) {
	body, contentType, err := a.fetch(ctx, rawURL)
	if err != nil {
		return "", err
	}
	key := keyPrefix + "/" + ulid.Make().String() + extensionFor(contentType)

	sourceURL := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		sourceURL = u.String() // percent-escaped, keeping S3's ASCII-only metadata happy
	}
	if _, err := a.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
		Metadata:    map[string]string{"source-url": truncate(sourceURL, 1024)},
	}); err != nil {
		return "", fmt.Errorf("store COA: %w", err)
	}
	return key, nil
}

// fetch downloads the COA and works out its content type.
func (a *S3Archiver) fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, "", fmt.Errorf("COA link %q is not an http(s) URL", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("build COA request: %w", err)
	}
	req.Header.Set("User-Agent", "grow-coa-archiver/1.0")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch COA: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch COA: %s returned %s", u.Host, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCOABytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read COA: %w", err)
	}
	if len(body) > maxCOABytes {
		return nil, "", fmt.Errorf("COA is larger than %d MB", maxCOABytes>>20)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("COA link returned an empty response")
	}

	// Trust a specific Content-Type; sniff the bytes when it's missing or
	// generic (some image hosts send JPGs as octet-stream), so the stored
	// copy opens in the browser instead of downloading.
	contentType := resp.Header.Get("Content-Type")
	if mt, _, _ := mime.ParseMediaType(contentType); mt == "" || mt == "application/octet-stream" || mt == "binary/octet-stream" {
		contentType = http.DetectContentType(body)
	}
	return body, contentType, nil
}

func (a *S3Archiver) Remove(ctx context.Context, key string) error {
	if _, err := a.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete archived COA: %w", err)
	}
	return nil
}

func extensionFor(contentType string) string {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch strings.ToLower(mt) {
	case "application/pdf":
		return ".pdf"
	case "text/html":
		return ".html"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
