package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Utils struct {
	MaxFileSize        int
	AllowedFileTypes   []string
	MaxJSONSize        int
	AllowUnknownFields bool
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

func (u *Utils) Slugify(text string) (string, error) {
	if text == "" {
		return "", errors.New("the input cannot be empty")
	}

	var regx = regexp.MustCompile(`[^a-z\d]+`)

	slug := strings.Trim(regx.ReplaceAllString(strings.ToLower(text), "-"), "-")

	if len(slug) == 0 {
		return "", errors.New("invalid input, the slug is empty")
	}

	return slug, nil
}

// Call this function as a goroutine
func (u *Utils) CtrlC(shutdownProcesses ...func()) {
	done := make(chan os.Signal, 2)

	signal.Notify(done, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("Running, press ctrl+c to quit")

	<-done

	fmt.Println("Shutting down...")

	for _, shutdownProcess := range shutdownProcesses {
		shutdownProcess()
	}

	os.Exit(0)
}

// Downloads a file and tries to force the browser to download it
func (u *Utils) DownloadStaticFile(w http.ResponseWriter, r *http.Request, p string, file string, displayName string) {
	filePath := path.Join(p, file)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))
	http.ServeFile(w, r, filePath)
}

type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (u *Utils) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1024 * 1024 // 1MB

	if u.MaxJSONSize > 0 {
		maxBytes = u.MaxJSONSize
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	decoder := json.NewDecoder(r.Body)

	if !u.AllowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	err := decoder.Decode(data)

	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at position %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at position %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown field %s", fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			return fmt.Errorf("error unmarshalling JSON: %v", err.Error())

		default:
			return err
		}
	}

	err = decoder.Decode(&struct{}{})

	if err != io.EOF {
		return errors.New("body must contain only one JSON object")
	}

	return nil
}

func (u *Utils) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	output, err := json.Marshal(data)

	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header().Set(key, value[0])
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(output)

	if err != nil {
		return err
	}

	return nil
}

func (u *Utils) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return u.WriteJSON(w, statusCode, payload)
}

func (u *Utils) PushJSONToRemote(uri string, data interface{}, client ...*http.Client) (*http.Response, int, error) {
	jsonData, err := json.Marshal(data)

	if err != nil {
		return nil, 0, err
	}

	httpClient := &http.Client{}

	if len(client) > 0 {
		httpClient = client[0]
	}

	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)

	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	return resp, resp.StatusCode, nil
}
