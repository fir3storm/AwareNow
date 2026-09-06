package models

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestAttachment(c *check.C) {
	ptx := PhishingTemplateContext{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
			Position:  "Space Janitor",
		},
		BaseURL:     "http://testurl.com",
		URL:         "http://testurl.com/?rid=1234567",
		TrackingURL: "http://testurl.local/track?rid=1234567",
		Tracker:     "<img alt='' style='display: none' src='http://testurl.local/track?rid=1234567'/>",
		From:        "From Address",
		RId:         "1234567",
	}

	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		log.Fatalf("Failed to open attachment folder 'testdata': %v\n", err)
	}
	for _, ff := range files {
		if !ff.IsDir() && !strings.Contains(ff.Name(), "templated") {
			fname := ff.Name()
			fmt.Printf("Checking attachment file -> %s\n", fname)
			data := readFile("testdata/" + fname)
			if filepath.Ext(fname) == ".b64" {
				fname = fname[:len(fname)-4]
			}
			a := Attachment{
				Content: data,
				Name:    fname,
			}
			t, err := a.ApplyTemplate(ptx)
			c.Assert(err, check.Equals, nil)
			c.Assert(a.vanillaFile, check.Equals, strings.Contains(fname, "without-vars"))
			c.Assert(a.vanillaFile, check.Not(check.Equals), strings.Contains(fname, "with-vars"))

			// Verify template was applied as expected.
			tt, err := ioutil.ReadAll(t)
			if err != nil {
				log.Fatalf("Failed to parse templated file '%s': %v\n", fname, err)
			}
			expectedOutput := readFile("testdata/" + strings.TrimSuffix(ff.Name(), filepath.Ext(ff.Name())) + ".templated" + filepath.Ext(ff.Name())) // e.g text-file-with-vars.templated.txt
			expectedBytes, err := base64.StdEncoding.DecodeString(expectedOutput)
			c.Assert(err, check.IsNil)
			switch filepath.Ext(fname) {
			case ".docx", ".docm", ".pptx", ".xlsx", ".xlsm":
				// ZIP compression can change between Go releases without changing
				// the document. Compare every entry's name and uncompressed bytes.
				c.Assert(attachmentArchiveContents(c, tt), check.DeepEquals, attachmentArchiveContents(c, expectedBytes), check.Commentf("attachment: %s", fname))
			default:
				c.Assert(tt, check.DeepEquals, expectedBytes, check.Commentf("attachment: %s", fname))
			}
		}
	}
}

func attachmentArchiveContents(c *check.C, data []byte) map[string][]byte {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	c.Assert(err, check.IsNil)
	contents := make(map[string][]byte, len(archive.File))
	for _, entry := range archive.File {
		_, duplicate := contents[entry.Name]
		c.Assert(duplicate, check.Equals, false, check.Commentf("duplicate archive entry: %s", entry.Name))
		reader, err := entry.Open()
		c.Assert(err, check.IsNil)
		content, readErr := ioutil.ReadAll(reader)
		closeErr := reader.Close()
		c.Assert(readErr, check.IsNil)
		c.Assert(closeErr, check.IsNil)
		contents[entry.Name] = content
	}
	return contents
}

func readFile(fname string) string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("Failed to open file '%s': %v\n", fname, err)
	}
	reader := bufio.NewReader(f)
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to read file '%s': %v\n", fname, err)
	}
	data := ""
	if filepath.Ext(fname) == ".b64" {
		data = string(content)
	} else {
		data = base64.StdEncoding.EncodeToString(content)
	}
	return data
}
