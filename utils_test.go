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
