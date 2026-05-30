package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"diplom.com/m/internal/ports"
)

// S3Store writes uploaded files to an S3-compatible object store such as MinIO.
type S3Store struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Client    *http.Client
}

func NewS3Store(endpoint, bucket, region, accessKey, secretKey string) (*S3Store, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("s3 endpoint is required")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("s3 bucket is required")
	}
	if strings.TrimSpace(region) == "" {
		region = "us-east-1"
	}
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("s3 credentials are required")
	}
	return &S3Store{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		Bucket:    bucket,
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (s *S3Store) Save(ctx context.Context, key string, r io.Reader) (int64, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}

	u, err := s.objectURL(key)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/octet-stream")

	payloadHash := sha256Hex(body)
	s.sign(req, payloadHash, time.Now().UTC())

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("s3 put object failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return int64(len(body)), nil
}

func (s *S3Store) Stat(ctx context.Context, key string) (ports.FileStatus, error) {
	u, err := s.objectURL(key)
	if err != nil {
		return ports.FileStatus{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return ports.FileStatus{}, err
	}

	payloadHash := sha256Hex(nil)
	s.sign(req, payloadHash, time.Now().UTC())

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return ports.FileStatus{}, err
	}
	defer resp.Body.Close()

	status := ports.FileStatus{
		Key:         key,
		Exists:      resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		ETag:        resp.Header.Get("ETag"),
	}
	if resp.ContentLength >= 0 {
		status.ContentLength = &resp.ContentLength
	}
	if v := resp.Header.Get("Last-Modified"); v != "" {
		if t, err := http.ParseTime(v); err == nil {
			status.LastModified = &t
		}
	}
	return status, nil
}

func (s *S3Store) Open(ctx context.Context, key string) (ports.FileObject, error) {
	u, err := s.objectURL(key)
	if err != nil {
		return ports.FileObject{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ports.FileObject{}, err
	}

	payloadHash := sha256Hex(nil)
	s.sign(req, payloadHash, time.Now().UTC())

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return ports.FileObject{}, err
	}

	status := responseStatus(key, resp)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return ports.FileObject{}, ports.ErrNotFound
		}
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ports.FileObject{}, fmt.Errorf("s3 get object failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return ports.FileObject{Status: status, Body: resp.Body}, nil
}

func (s *S3Store) objectURL(key string) (*url.URL, error) {
	base, err := url.Parse(s.Endpoint)
	if err != nil {
		return nil, err
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid s3 endpoint %q", s.Endpoint)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + s.Bucket + "/" + strings.TrimLeft(key, "/")
	base.RawPath = ""
	base.RawQuery = ""
	return base, nil
}

func responseStatus(key string, resp *http.Response) ports.FileStatus {
	status := ports.FileStatus{
		Key:         key,
		Exists:      resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		ETag:        resp.Header.Get("ETag"),
	}
	if resp.ContentLength >= 0 {
		status.ContentLength = &resp.ContentLength
	}
	if v := resp.Header.Get("Last-Modified"); v != "" {
		if t, err := http.ParseTime(v); err == nil {
			status.LastModified = &t
		}
	}
	return status
}

func (s *S3Store) sign(req *http.Request, payloadHash string, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")

	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", amzDate)

	headers := map[string]string{
		"host":                 req.URL.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	signedHeaders := signedHeaderNames(headers)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		req.URL.RawQuery,
		canonicalHeaders(headers),
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{date, s.Region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(signingKey(s.SecretKey, date, s.Region), stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.AccessKey,
		scope,
		signedHeaders,
		signature,
	))
}

func (s *S3Store) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func canonicalURI(u *url.URL) string {
	if u.EscapedPath() == "" {
		return "/"
	}
	return u.EscapedPath()
}

func canonicalHeaders(headers map[string]string) string {
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(strings.Join(strings.Fields(headers[name]), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

func signedHeaderNames(headers map[string]string) string {
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ";")
}

func signingKey(secret, date, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var _ ports.FileStore = (*S3Store)(nil)
