package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type HTTPClient struct {
	URL       string
	Username  string
	Password  string
	Proxy     string
	Transport *http.Transport
}

func (h *HTTPClient) Call(context context.Context, input *[]Input, result *[]Output) error {
	return h.call(context, input, result)
}

func (h *HTTPClient) CallSingle(ctx context.Context, method string, params interface{}, result interface{}) error {
	return CallSingle(h, ctx, method, params, result)
}

func (h *HTTPClient) call(context context.Context, input, result interface{}) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context, "POST", h.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	if h.Username != "" && h.Password != "" {
		req.SetBasicAuth(h.Username, h.Password)
	}
	var transport *http.Transport
	if h.Transport == nil {
		transport = &http.Transport{}
	} else {
		transport = h.Transport
	}
	if h.Proxy != "" {
		proxyURL, err := url.Parse(h.Proxy)
		if err != nil {
			return err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	httpClient := &http.Client{Transport: transport}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, err = io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf(string(body))
	}
	return json.NewDecoder(res.Body).Decode(result)
}
