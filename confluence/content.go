package confluence

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Content struct {
	Id     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Body   struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
}

func (w *Wiki) contentEndpoint(contentID string) (*url.URL, error) {
	return url.ParseRequestURI(w.endPoint.String() + "/content/" + contentID)
}

func (w *Wiki) DeleteContent(contentID string) error {
	contentEndPoint, err := w.contentEndpoint(contentID)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", contentEndPoint.String(), nil)
	if err != nil {
		return err
	}

	_, err = w.sendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wiki) GetContent(contentID string, expand []string) (*Content, error) {
	contentEndPoint, err := w.contentEndpoint(contentID)
	if err != nil {
		return nil, err
	}
	data := url.Values{}
	data.Set("expand", strings.Join(expand, ","))
	contentEndPoint.RawQuery = data.Encode()

	req, err := http.NewRequest("GET", contentEndPoint.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var content Content
	err = json.Unmarshal(res, &content)
	if err != nil {
		return nil, err
	}

	return &content, nil
}

func (w *Wiki) UpdateContent(content *Content) (*Content, error) {
	jsonbody, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	contentEndPoint, err := w.contentEndpoint(content.Id)
	req, err := http.NewRequest("PUT", contentEndPoint.String(), strings.NewReader(string(jsonbody)))
	req.Header.Add("Content-Type", "application/json")

	res, err := w.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var newContent Content
	err = json.Unmarshal(res, &newContent)
	if err != nil {
		return nil, err
	}

	return &newContent, nil
}

// https://docs.atlassian.com/atlassian-confluence/REST/6.5.2/#content/{id}/child/attachment
func (w *Wiki) AddAttachments(content *Content, files []string) error {
	contentEndPoint, err := w.contentEndpoint(content.Id)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, path := range files {
		err = func() error {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			fi, err := file.Stat()
			if err != nil {
				return err
			}

			part, err := writer.CreateFormFile("file", fi.Name())
			if err != nil {
				return err
			}

			_, err = io.Copy(part, file)
			return err
		}()
		if err != nil {
			return err
		}
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST",
		contentEndPoint.String()+"/child/attachment", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err = w.sendRequest(req)
	if err != nil {
		return err
	}

	return nil
}
