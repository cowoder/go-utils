package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestUtils_RandomString(t *testing.T) {
	var testUtils Utils

	s := testUtils.RandomString(32)

	if len(s) != 32 {
		t.Errorf("RandomString() = %s; want length 32", s)
	}
}

var uploadTests = []struct {
	name             string
	allowedFileTypes []string
	renameFile       bool
	errorExpected    bool
}{
	{name: "allowed no rename", allowedFileTypes: []string{"image/jpeg", "image/png"}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedFileTypes: []string{"image/jpeg", "image/png"}, renameFile: true, errorExpected: false},
	{name: "not allowed", allowedFileTypes: []string{"image/jpeg"}, renameFile: false, errorExpected: true},
}

func TestUtils_UploadFiles(t *testing.T) {
	for _, e := range uploadTests {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			part, err := writer.CreateFormFile("file", "./testdata/img.png")

			if err != nil {
				t.Error("error creating form", err)
			}

			f, err := os.Open("./testdata/img.png")

			if err != nil {
				t.Error("error opening file", err)
			}

			defer f.Close()

			img, _, err := image.Decode(f)

			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)

			if err != nil {
				t.Error("error encoding image", err)
			}
		}()

		req := httptest.NewRequest("POST", "/", pr)

		req.Header.Add("Content-Type", writer.FormDataContentType())

		var testUtils Utils

		testUtils.AllowedFileTypes = e.allowedFileTypes

		uploadedFiles, err := testUtils.UploadFiles(req, "./testdata/uploads/", e.renameFile)

		if err != nil && !e.errorExpected {
			t.Error("error uploading file", err)
		}

		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("file %s does not exist: %s", uploadedFiles[0].NewFileName, err.Error())
			}

			_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if !e.errorExpected && err != nil {
			t.Errorf("%s: error expected but none received", e.name)
		}

		wg.Wait()
	}
}

func TestUtils_UploadFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "./testdata/img.png")

		if err != nil {
			t.Error("error creating form", err)
		}

		f, err := os.Open("./testdata/img.png")

		if err != nil {
			t.Error("error opening file", err)
		}

		defer f.Close()

		img, _, err := image.Decode(f)

		if err != nil {
			t.Error("error decoding image", err)
		}

		err = png.Encode(part, img)

		if err != nil {
			t.Error("error encoding image", err)
		}
	}()

	req := httptest.NewRequest("POST", "/", pr)

	req.Header.Add("Content-Type", writer.FormDataContentType())

	var testUtils Utils

	uploadedFile, err := testUtils.UploadFile(req, "./testdata/uploads/", true)

	if err != nil {
		t.Error("error uploading file", err)
	}

	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
		t.Errorf("file does not exist: %s", err.Error())
	}

	_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName))
}

func TestUtils_CreateDirIfNotExists(t *testing.T) {
	var testUtils Utils

	err := testUtils.CreateDirIfNotExists("./testdata/testdir")

	if err != nil {
		t.Error("error creating dir", err)
	}

	err = testUtils.CreateDirIfNotExists("./testdata/testdir")

	if err != nil {
		t.Error("error creating dir", err)
	}

	_ = os.Remove("./testdata/testdir")
}

var slugifyTests = []struct {
	name          string
	input         string
	expected      string
	errorExpected bool
}{
	{name: "valid string", input: "Hello World!", expected: "hello-world", errorExpected: false},
	{name: "valid string with numbers", input: "Hello World 123!", expected: "hello-world-123", errorExpected: false},
	{name: "empty string", input: "", expected: "", errorExpected: true},
	{name: "string with special characters", input: "Hello World!@#$%^&*()", expected: "hello-world", errorExpected: false},
	{name: "japanese string", input: "こんにちは世界", expected: "", errorExpected: true},
	{name: "mix of roman and japanese", input: "こんにちは世界 Hello World!", expected: "hello-world", errorExpected: false},
}

func TestUtils_Slugify(t *testing.T) {
	var testUtils Utils

	for _, e := range slugifyTests {
		slug, err := testUtils.Slugify(e.input)

		if err != nil && !e.errorExpected {
			t.Errorf("%s: error received but none expected: %s", e.name, err.Error())
		}

		if !e.errorExpected && err != nil {
			t.Errorf("%s: error expected but none received", e.name)
		}

		if !e.errorExpected && slug != e.expected {
			t.Errorf("%s: expected %s; got %s", e.name, e.expected, slug)
		}
	}
}

func TestUtils_CtrlC(t *testing.T) {}

func TestUtils_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()

	req := httptest.NewRequest("GET", "/", nil)

	var testUtils Utils

	testUtils.DownloadStaticFile(rr, req, "./testdata", "img.png", "test.png")

	res := rr.Result()

	defer res.Body.Close()

	if res.Header["Content-Length"][0] != "30691" {
		t.Error("invalid content length of:", res.Header["Content-Length"][0])
	}

	if res.Header["Content-Disposition"][0] != "attachment; filename=\"test.png\"" {
		t.Error("invalid content disposition of:", res.Header["Content-Disposition"][0])
	}

	_, err := io.ReadAll(res.Body)

	if err != nil {
		t.Error("error reading response body", err)
	}
}

var jsonTests = []struct {
	name               string
	input              string
	errorExpected      bool
	maxSize            int
	allowUnknownFields bool
}{
	{name: "valid json", input: `{"name": "John Doe"}`, errorExpected: false, maxSize: 1024, allowUnknownFields: false},
	{name: "invalid json", input: `{"name":`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "incorrect type", input: `{"name": 123`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "two json files", input: `{"name": "John Doe"}{"name": "John Doe"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "empty body", input: ``, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "syntax error", input: `{"name": John Doe"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "unknown field", input: `{"full_name": "John Doe"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: false},
	{name: "allow unknown field", input: `{"full_name": "John Doe"}`, errorExpected: false, maxSize: 1024, allowUnknownFields: true},
	{name: "missing field name", input: `{name: "John Doe"}`, errorExpected: true, maxSize: 1024, allowUnknownFields: true},
	{name: "file too large", input: `{"name": "John Doe"}`, errorExpected: true, maxSize: 1, allowUnknownFields: true},
	{name: "not json", input: `"name: John Doe"`, errorExpected: true, maxSize: 1024, allowUnknownFields: true},
}

func TestUtils_ReadJSON(t *testing.T) {
	var testUtils Utils

	for _, e := range jsonTests {
		testUtils.MaxJSONSize = e.maxSize
		testUtils.AllowUnknownFields = e.allowUnknownFields

		var decodedJSON struct {
			Name string `json:"name"`
		}

		req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(e.input)))

		rr := httptest.NewRecorder()

		err := testUtils.ReadJSON(rr, req, &decodedJSON)

		if e.errorExpected && err == nil {
			t.Errorf("%s: error expected but none received", e.name)
		}

		if !e.errorExpected && err != nil {
			t.Errorf("%s: error received but none expected: %s", e.name, err.Error())
		}

		req.Body.Close()
	}
}

func TestUtils_WriteJSON(t *testing.T) {
	var testUtils Utils

	rr := httptest.NewRecorder()

	payload := JSONResponse{
		Error:   false,
		Message: "Hello World!",
	}

	headers := make(http.Header)
	headers.Add("HELLO", "WORLD")

	err := testUtils.WriteJSON(rr, http.StatusOK, payload, headers)

	if err != nil {
		t.Error("error writing json", err)
	}
}

func TestUtils_ErrorJSON(t *testing.T) {
	var testUtils Utils

	rr := httptest.NewRecorder()

	err := testUtils.ErrorJSON(rr, errors.New("dummy error"), http.StatusServiceUnavailable)

	if err != nil {
		t.Error("error writing json", err)
	}

	var response JSONResponse

	decoder := json.NewDecoder(rr.Body)

	err = decoder.Decode(&response)

	if err != nil {
		t.Error("error decoding json", err)
	}

	if !response.Error {
		t.Error("error expected but none received")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status code %d; got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestUtils_PushJSONToRemote(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})

	var testUtils Utils

	var payload struct {
		Data string `json:"data"`
	}

	payload.Data = "Hello World!"

	_, _, err := testUtils.PushJSONToRemote("http://example.com/some", payload, client)

	if err != nil {
		t.Error("error pushing json to remote", err)
	}
}
