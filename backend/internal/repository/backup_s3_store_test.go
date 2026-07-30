package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

type backupS3Mock struct {
	putObject               func(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	createMultipartUpload   func(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	uploadPart              func(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	completeMultipartUpload func(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	abortMultipartUpload    func(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

func (m *backupS3Mock) PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObject == nil {
		return nil, errors.New("unexpected PutObject")
	}
	return m.putObject(ctx, input, opts...)
}

func (m *backupS3Mock) CreateMultipartUpload(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	if m.createMultipartUpload == nil {
		return nil, errors.New("unexpected CreateMultipartUpload")
	}
	return m.createMultipartUpload(ctx, input, opts...)
}

func (m *backupS3Mock) UploadPart(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	if m.uploadPart == nil {
		return nil, errors.New("unexpected UploadPart")
	}
	return m.uploadPart(ctx, input, opts...)
}

func (m *backupS3Mock) CompleteMultipartUpload(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	if m.completeMultipartUpload == nil {
		return nil, errors.New("unexpected CompleteMultipartUpload")
	}
	return m.completeMultipartUpload(ctx, input, opts...)
}

func (m *backupS3Mock) AbortMultipartUpload(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	if m.abortMultipartUpload == nil {
		return nil, errors.New("unexpected AbortMultipartUpload")
	}
	return m.abortMultipartUpload(ctx, input, opts...)
}

func (m *backupS3Mock) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, errors.New("unexpected GetObject")
}

func (m *backupS3Mock) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("unexpected DeleteObject")
}

func (m *backupS3Mock) HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return nil, errors.New("unexpected HeadBucket")
}

type generatedBackupReader struct {
	remaining int64
	readBytes int64
	maxRead   int
}

func (r *generatedBackupReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		r.maxRead = len(p)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remaining {
		n = int(r.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = byte((r.readBytes + int64(i)) % 251)
	}
	r.remaining -= int64(n)
	r.readBytes += int64(n)
	return n, nil
}

type failingBackupReader struct {
	generatedBackupReader
	err error
}

func (r *failingBackupReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, r.err
	}
	return r.generatedBackupReader.Read(p)
}

func TestS3BackupStoreUploadSmallObjectUsesPutObject(t *testing.T) {
	tests := map[string][]byte{
		"empty": nil,
		"small": []byte("small backup"),
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			var uploaded bytes.Buffer
			var putLength int64
			mock := &backupS3Mock{
				putObject: func(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					_, err := io.Copy(&uploaded, input.Body)
					putLength = aws.ToInt64(input.ContentLength)
					return &s3.PutObjectOutput{}, err
				},
			}
			store := &S3BackupStore{client: mock, bucket: "backups"}

			size, err := store.Upload(context.Background(), "backup.gz", bytes.NewReader(content), "application/gzip")

			require.NoError(t, err)
			require.Equal(t, int64(len(content)), size)
			require.Equal(t, content, uploaded.Bytes())
			require.Equal(t, int64(len(content)), putLength)
		})
	}
}

func TestS3BackupStoreUploadMultipartStreamsFixedSizeParts(t *testing.T) {
	reader := &generatedBackupReader{remaining: int64(backupMultipartPartSize*2 + 123)}
	var partNumbers []int32
	var partSizes []int64
	var readAtUpload []int64
	var completed []types.CompletedPart
	mock := &backupS3Mock{
		createMultipartUpload: func(_ context.Context, _ *s3.CreateMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{UploadId: aws.String("upload-1")}, nil
		},
		uploadPart: func(_ context.Context, input *s3.UploadPartInput, _ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
			partSize, err := io.Copy(io.Discard, input.Body)
			if err != nil {
				return nil, err
			}
			partNumber := aws.ToInt32(input.PartNumber)
			partNumbers = append(partNumbers, partNumber)
			partSizes = append(partSizes, partSize)
			readAtUpload = append(readAtUpload, reader.readBytes)
			return &s3.UploadPartOutput{ETag: aws.String(fmt.Sprintf("etag-%d", partNumber))}, nil
		},
		completeMultipartUpload: func(_ context.Context, input *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
			completed = input.MultipartUpload.Parts
			return &s3.CompleteMultipartUploadOutput{}, nil
		},
	}
	store := &S3BackupStore{client: mock, bucket: "backups"}

	size, err := store.Upload(context.Background(), "backup.gz", reader, "application/gzip")

	require.NoError(t, err)
	require.Equal(t, int64(backupMultipartPartSize*2+123), size)
	require.Equal(t, []int32{1, 2, 3}, partNumbers)
	require.Equal(t, []int64{backupMultipartPartSize, backupMultipartPartSize, 123}, partSizes)
	require.Equal(t, []int64{backupMultipartPartSize, backupMultipartPartSize * 2, backupMultipartPartSize*2 + 123}, readAtUpload)
	require.Len(t, completed, 3)
	require.LessOrEqual(t, reader.maxRead, backupMultipartPartSize)
}

func TestS3BackupStoreUploadMultipartAbortsOnFailure(t *testing.T) {
	tests := []struct {
		name          string
		reader        io.Reader
		partError     error
		completeError error
	}{
		{name: "read", reader: &failingBackupReader{generatedBackupReader: generatedBackupReader{remaining: backupMultipartPartSize}, err: errors.New("reader failed")}},
		{name: "part", reader: &generatedBackupReader{remaining: backupMultipartPartSize * 2}, partError: errors.New("part failed")},
		{name: "complete", reader: &generatedBackupReader{remaining: backupMultipartPartSize * 2}, completeError: errors.New("complete failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abortCalls := 0
			mock := &backupS3Mock{
				createMultipartUpload: func(_ context.Context, _ *s3.CreateMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
					return &s3.CreateMultipartUploadOutput{UploadId: aws.String("upload-1")}, nil
				},
				uploadPart: func(_ context.Context, input *s3.UploadPartInput, _ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
					_, err := io.Copy(io.Discard, input.Body)
					if err != nil {
						return nil, err
					}
					if tt.partError != nil {
						return nil, tt.partError
					}
					return &s3.UploadPartOutput{ETag: aws.String("etag")}, nil
				},
				completeMultipartUpload: func(_ context.Context, _ *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
					return nil, tt.completeError
				},
				abortMultipartUpload: func(_ context.Context, _ *s3.AbortMultipartUploadInput, _ ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
					abortCalls++
					return &s3.AbortMultipartUploadOutput{}, nil
				},
			}
			store := &S3BackupStore{client: mock, bucket: "backups"}

			_, err := store.Upload(context.Background(), "backup.gz", tt.reader, "application/gzip")

			require.Error(t, err)
			require.Equal(t, 1, abortCalls)
		})
	}
}
