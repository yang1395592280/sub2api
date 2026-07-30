package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	backupMultipartPartSize = 16 * 1024 * 1024
	backupMultipartMaxParts = 10_000
	backupAbortTimeout      = 30 * time.Second
)

type s3BackupAPI interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

// S3BackupStore implements service.BackupObjectStore using AWS S3 compatible storage
type S3BackupStore struct {
	client        s3BackupAPI
	presignClient *s3.PresignClient
	bucket        string
}

// NewS3BackupStoreFactory returns a BackupObjectStoreFactory that creates S3-backed stores
func NewS3BackupStoreFactory() service.BackupObjectStoreFactory {
	return func(ctx context.Context, cfg *service.BackupS3Config) (service.BackupObjectStore, error) {
		client, err := newS3Client(ctx, s3ClientParams{
			Endpoint:        cfg.Endpoint,
			Region:          cfg.Region,
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			ForcePathStyle:  cfg.ForcePathStyle,
		})
		if err != nil {
			return nil, err
		}
		return &S3BackupStore{
			client:        client,
			presignClient: s3.NewPresignClient(client),
			bucket:        cfg.Bucket,
		}, nil
	}
}

func (s *S3BackupStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) (int64, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	defer finish()

	partBuffer := make([]byte, backupMultipartPartSize)
	firstPartSize, readErr := io.ReadFull(body, partBuffer)
	if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
		return s.putObject(ctx, key, contentType, partBuffer[:firstPartSize])
	}
	if readErr != nil {
		return 0, fmt.Errorf("read first backup part: %w", readErr)
	}

	createOutput, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      &s.bucket,
		Key:         &key,
		ContentType: &contentType,
	})
	if err != nil {
		return 0, fmt.Errorf("S3 CreateMultipartUpload: %w", err)
	}
	if createOutput == nil || createOutput.UploadId == nil || *createOutput.UploadId == "" {
		return 0, fmt.Errorf("S3 CreateMultipartUpload: response missing upload ID")
	}

	uploadID := createOutput.UploadId
	completedParts := make([]types.CompletedPart, 0, 8)
	totalSize := int64(0)

	uploadPart := func(partNumber int32, data []byte) error {
		partSize := int64(len(data))
		output, uploadErr := s.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:        &s.bucket,
			Key:           &key,
			UploadId:      uploadID,
			PartNumber:    aws.Int32(partNumber),
			Body:          bytes.NewReader(data),
			ContentLength: aws.Int64(partSize),
		})
		if uploadErr != nil {
			return fmt.Errorf("S3 UploadPart %d: %w", partNumber, uploadErr)
		}
		if output == nil || output.ETag == nil || *output.ETag == "" {
			return fmt.Errorf("S3 UploadPart %d: response missing ETag", partNumber)
		}
		completedParts = append(completedParts, types.CompletedPart{
			ETag:       output.ETag,
			PartNumber: aws.Int32(partNumber),
		})
		totalSize += partSize
		return nil
	}

	if err := uploadPart(1, partBuffer[:firstPartSize]); err != nil {
		s.abortMultipartUpload(ctx, key, uploadID)
		return 0, err
	}

	for partNumber := int32(2); ; partNumber++ {
		partSize, partReadErr := io.ReadFull(body, partBuffer)
		if partReadErr != nil && partReadErr != io.EOF && partReadErr != io.ErrUnexpectedEOF {
			s.abortMultipartUpload(ctx, key, uploadID)
			return 0, fmt.Errorf("read backup part %d: %w", partNumber, partReadErr)
		}
		if partSize == 0 {
			break
		}
		if partNumber > backupMultipartMaxParts {
			s.abortMultipartUpload(ctx, key, uploadID)
			return 0, fmt.Errorf("backup exceeds S3 multipart limit of %d parts", backupMultipartMaxParts)
		}
		if err := uploadPart(partNumber, partBuffer[:partSize]); err != nil {
			s.abortMultipartUpload(ctx, key, uploadID)
			return 0, err
		}
		if partReadErr == io.ErrUnexpectedEOF {
			break
		}
	}

	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &s.bucket,
		Key:      &key,
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		s.abortMultipartUpload(ctx, key, uploadID)
		return 0, fmt.Errorf("S3 CompleteMultipartUpload: %w", err)
	}
	return totalSize, nil
}

func (s *S3BackupStore) putObject(ctx context.Context, key string, contentType string, data []byte) (int64, error) {
	contentLength := int64(len(data))
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          bytes.NewReader(data),
		ContentType:   &contentType,
		ContentLength: aws.Int64(contentLength),
	})
	if err != nil {
		return 0, fmt.Errorf("S3 PutObject: %w", err)
	}
	return contentLength, nil
}

func (s *S3BackupStore) abortMultipartUpload(ctx context.Context, key string, uploadID *string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backupAbortTimeout)
	defer cancel()
	_, _ = s.client.AbortMultipartUpload(cleanupCtx, &s3.AbortMultipartUploadInput{
		Bucket:   &s.bucket,
		Key:      &key,
		UploadId: uploadID,
	})
}

func (s *S3BackupStore) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject: %w", err)
	}
	return result.Body, nil
}

func (s *S3BackupStore) Delete(ctx context.Context, key string) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	return err
}

func (s *S3BackupStore) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// 强制 attachment disposition：浏览器同页导航该 URL 时直接触发下载而非渲染，
	// 前端无需依赖会被弹窗拦截的新标签页。
	disposition := fmt.Sprintf("attachment; filename=%q", path.Base(key))
	result, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     &s.bucket,
		Key:                        &key,
		ResponseContentDisposition: &disposition,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return result.URL, nil
}

func (s *S3BackupStore) HeadBucket(ctx context.Context) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &s.bucket,
	})
	finish()
	if err != nil {
		return fmt.Errorf("S3 HeadBucket failed: %w", err)
	}
	return nil
}
