package utils

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Utils struct {
	MaxFileSize      int
	AllowedFileTypes []string
}

// RandomString generates a random string of length n
func (u *Utils) RandomString(length int) string {
	b := make([]byte, length)

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

func (u *Utils) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true

	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	if u.MaxFileSize == 0 {
		u.MaxFileSize = 1024 * 1024 * 1024 // 1GB
	}

	err := u.CreateDirIfNotExists(uploadDir)

	if err != nil {
		return nil, err
	}

	err = r.ParseMultipartForm(int64(u.MaxFileSize))

	if err != nil {
		return nil, errors.New("the uploaded file exceeds the maximum file size")
	}

	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile

				inputFile, err := fileHeader.Open()

				if err != nil {
					return nil, err
				}

				defer inputFile.Close()

				buffer := make([]byte, 512)
				_, err = inputFile.Read(buffer)

				if err != nil {
					return nil, err
				}

				allowed := false
				fileType := http.DetectContentType(buffer)

				if len(u.AllowedFileTypes) > 0 {
					for _, allowedFileType := range u.AllowedFileTypes {
						if strings.EqualFold(fileType, allowedFileType) {
							allowed = true
						}
					}
				} else {
					allowed = true
				}

				if !allowed {
					return nil, errors.New("the uploaded file type is not allowed")
				}

				_, err = inputFile.Seek(0, 0)

				if err != nil {
					return nil, err
				}

				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", u.RandomString(32), filepath.Ext(fileHeader.Filename))
				} else {
					uploadedFile.NewFileName = fileHeader.Filename
				}

				uploadedFile.OriginalFileName = fileHeader.Filename

				var outputFile *os.File

				defer outputFile.Close()

				if outputFile, err := os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outputFile, inputFile)

					if err != nil {
						return nil, err
					}

					uploadedFile.FileSize = fileSize
				}

				uploadedFiles = append(uploadedFiles, &uploadedFile)

				return uploadedFiles, nil
			}(uploadedFiles)

			if err != nil {
				return uploadedFiles, err
			}
		}
	}

	return uploadedFiles, nil
}

func (u *Utils) UploadFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true

	if len(rename) > 0 {
		renameFile = rename[0]
	}

	files, err := u.UploadFiles(r, uploadDir, renameFile)

	if err != nil {
		return nil, err
	}

	return files[0], nil
}

func (u *Utils) CreateDirIfNotExists(dir string) error {
	const mode = 0755

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, mode)

		if err != nil {
			return err
		}
	}

	return nil
}
