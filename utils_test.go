package utils

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

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
