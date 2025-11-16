package detector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"
)

type Box struct {
	Label   string  `json:"label"`
	ClassID int     `json:"class_id"`
	Conf    float64 `json:"conf"`
	X1      int     `json:"x1"`
	Y1      int     `json:"y1"`
	X2      int     `json:"x2"`
	Y2      int     `json:"y2"`
}

type Response struct {
	Boxes []Box `json:"boxes"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) DetectJPEG(jpeg []byte, conf, iou float64) ([]Box, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "frame.jpg")
	if err != nil {
		return nil, err
	}
	if _, err = fw.Write(jpeg); err != nil {
		return nil, err
	}
	_ = w.Close()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/detect?conf=%g&iou=%g", c.baseURL, conf, iou), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Boxes, nil
}
