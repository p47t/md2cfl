package confluence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
)

// https://docs.atlassian.com/atlassian-confluence/REST/6.5.2/#content/{id}/child/attachment

type Attachments struct {
	Results []Attachment `json:"results"`
	Size    int          `json:"size"`
}

type Attachment struct {
	Id       string `json:"id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Title    string `json:"title"`
	Metadata struct {
		Comment   string `json:"comment"`
		MediaType string `json:"mediaType"`
	} `json:"metadata"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
}

func (w *Wiki) newAttachmentEndpoint(contentID string) (*url.URL, error) {
	return url.ParseRequestURI(w.endPoint.String() + "/content/" + contentID + "/child/attachment")
}

func (w *Wiki) attachmentEndpoint(contentID, attachmentID string) (*url.URL, error) {
	if endpoint, err := w.newAttachmentEndpoint(contentID); err == nil {
		return url.ParseRequestURI(endpoint.String() + "/" + attachmentID)
	} else {
		return nil, err
	}
}

func (w *Wiki) attachmentDataEndpoint(contentID, attachmentID string) (*url.URL, error) {
	if endpoint, err := w.attachmentEndpoint(contentID, attachmentID); err == nil {
		return url.ParseRequestURI(endpoint.String() + "/data")
	} else {
		return nil, err
	}
}

func (w *Wiki) DeleteAttachment(contentID string, attachmentID string) error {
	endpoint, err := w.attachmentEndpoint(contentID, attachmentID)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", endpoint.String(), nil)
	if err != nil {
		return err
	}

	_, err = w.sendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wiki) GetAttachment(contentID, attachmentID string) (*Attachment, error) {
	endpoint, err := w.attachmentEndpoint(contentID, attachmentID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var attachments Attachments
	err = json.Unmarshal(res, &attachments)
	if err != nil {
		return nil, err
	}
	if len(attachments.Results) < 1 {
		return nil, fmt.Errorf("empty list")
	}

	return &attachments.Results[0], nil
}

func (w *Wiki) GetAttachmentByFilename(contentID, filename string) (*Attachment, error) {
	endpoint, err := w.newAttachmentEndpoint(contentID)
	if err != nil {
		return nil, err
	}
	data := url.Values{}
	data.Set("filename", filename)
	endpoint.RawQuery = data.Encode()

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var attachments Attachments
	err = json.Unmarshal(res, &attachments)
	if err != nil {
		return nil, err
	}
	if len(attachments.Results) < 1 {
		return nil, fmt.Errorf("empty list")
	}

	return &attachments.Results[0], nil
}

func (w *Wiki) UpdateAttachment(contentID, attachmentID, path string, minorEdit bool) (*Attachment, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	part, err := writer.CreateFormFile("file", fi.Name())
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	writer.WriteField("minorEdit", strconv.FormatBool(minorEdit))
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	endpoint, err := w.attachmentDataEndpoint(contentID, attachmentID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpoint.String(), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var attachment Attachment
	err = json.Unmarshal(res, &attachment)
	if err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (w *Wiki) AddAttachment(contentID, path string) (*Attachment, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	part, err := writer.CreateFormFile("file", fi.Name())
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	endpoint, err := w.newAttachmentEndpoint(contentID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpoint.String(), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var attachments Attachments
	err = json.Unmarshal(res, &attachments)
	if err != nil {
		return nil, err
	}
	if len(attachments.Results) < 1 {
		return nil, fmt.Errorf("empty list")
	}

	return &attachments.Results[0], nil
}

func (w *Wiki) AddUpdateAttachments(contentID string, files []string, progress func(msg string)) ([]*Attachment, []error) {
	var results []*Attachment
	var errors []error
	for _, f := range files {
		filename := path.Base(f)
		attachment, err := w.GetAttachmentByFilename(contentID, filename)
		if err != nil {
			progress(fmt.Sprint("Adding new attachment", filename))
			attachment, err = w.AddAttachment(contentID, f)
		} else {
			progress(fmt.Sprint("Updating attachment", filename, attachment.Id))
			attachment, err = w.UpdateAttachment(contentID, attachment.Id, f, true)
		}
		if err == nil {
			results = append(results, attachment)
		} else {
			errors = append(errors, fmt.Errorf("failed to update attachment %s: %s", f, err.Error()))
		}
	}
	return results, errors
}
