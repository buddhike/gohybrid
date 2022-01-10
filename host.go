package gohybrid

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

type bufferedResponse struct {
	header      http.Header
	wroteHeader bool
	status      int
	buffer      *bytes.Buffer
}

func (w *bufferedResponse) Header() http.Header {
	return w.header
}

func (w *bufferedResponse) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.buffer.Write(p)
}

func (w *bufferedResponse) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.status = statusCode
		w.wroteHeader = true
	}
}

type lambdaResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
	Body              string              `json:"body"`
}

type httpAdapterHandler struct {
	http http.Handler
}

func (h *httpAdapterHandler) Invoke(ctx context.Context, req []byte) ([]byte, error) {
	m := make(map[string]interface{})
	err := json.Unmarshal(req, &m)
	if err != nil {
		return nil, err
	}
	log.Println(string(req))
	if rctx, ok := m["requestContext"]; ok {
		k := rctx.(map[string]interface{})
		if _, s := k["elb"]; s {
			return h.handleALBEvent(ctx, m)
		} else if _, s := k["http"]; s {
			return h.handleAPIGatewayHTTPEvent(ctx, m)
		} else if _, s := k["resourcePath"]; s {
			return h.handleAPIGatewayEvent(ctx, m)
		}
	}
	return nil, errors.New("unsupported integration, supported integrations are: ALB, API Gateway")
}

func (h *httpAdapterHandler) handleALBEvent(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["path"].(string)
	method := event["httpMethod"].(string)
	body := h.extractBodyReader(event)
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	h.mapQueryString(event, req)
	h.mapHeaders(event, req)
	res := &bufferedResponse{
		header: make(http.Header),
		buffer: &bytes.Buffer{},
	}
	h.http.ServeHTTP(res, req)
	return json.Marshal(h.toLambdaResponse(res))
}

func (h *httpAdapterHandler) handleAPIGatewayEvent(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["path"].(string)
	method := event["httpMethod"].(string)
	body := h.extractBodyReader(event)
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	h.mapQueryString(event, req)
	h.mapHeaders(event, req)
	res := &bufferedResponse{
		header: make(http.Header),
		buffer: &bytes.Buffer{},
	}
	h.http.ServeHTTP(res, req)
	return json.Marshal(h.toLambdaResponse(res))
}

func (h *httpAdapterHandler) handleAPIGatewayHTTPEvent(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["rawPath"].(string)
	rctx := event["requestContext"].(map[string]interface{})
	httpInfo := rctx["http"].(map[string]interface{})
	method := httpInfo["method"].(string)
	body := h.extractBodyReader(event)
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	h.mapQueryString(event, req)
	h.mapHeaders(event, req)
	res := &bufferedResponse{
		header: make(http.Header),
		buffer: &bytes.Buffer{},
	}
	h.http.ServeHTTP(res, req)
	return json.Marshal(h.toLambdaResponse(res))
}

func (h *httpAdapterHandler) extractBodyReader(event map[string]interface{}) io.Reader {
	if body, ok := event["body"].(string); ok {
		isBase64Encoded := event["isBase64Encoded"].(bool)
		if isBase64Encoded {
			return base64.NewDecoder(&base64.Encoding{}, bytes.NewBufferString(body))
		}
		return bytes.NewBufferString(body)
	}
	return &bytes.Reader{}
}

func (h *httpAdapterHandler) mapQueryString(event map[string]interface{}, req *http.Request) {
	q := req.URL.Query()
	if p, ok := event["queryStringParameters"]; ok && p != nil {
		for k, v := range p.(map[string]interface{}) {
			q.Add(k, v.(string))
		}
	}
	if p, ok := event["multiValueQueryStringParameters"]; ok && p != nil {
		for k, v := range p.(map[string]interface{}) {
			for _, i := range v.([]interface{}) {
				q.Add(k, i.(string))
			}
		}
	}
}

func (h *httpAdapterHandler) mapHeaders(event map[string]interface{}, req *http.Request) {
	if p, ok := event["headers"]; ok && p != nil {
		for k, v := range p.(map[string]interface{}) {
			req.Header.Add(k, v.(string))
		}
	}
	if p, ok := event["multiValueHeaders"]; ok && p != nil {
		for k, v := range p.(map[string]interface{}) {
			d := toStringSlice(v.([]interface{}))
			j := strings.Join(d, ",")
			req.Header.Add(k, j)
		}
	}
}

func (h *httpAdapterHandler) toLambdaResponse(res *bufferedResponse) *lambdaResponse {
	r := &lambdaResponse{
		StatusCode:        res.status,
		Headers:           map[string]string{},
		MultiValueHeaders: map[string][]string{},
		IsBase64Encoded:   false,
		Body:              res.buffer.String(),
	}
	for k, v := range res.header {
		if len(v) == 1 {
			r.Headers[k] = v[0]
		} else {
			r.MultiValueHeaders[k] = v
		}
	}
	return r
}

func toStringSlice(s []interface{}) []string {
	d := make([]string, len(s))
	for i := 0; i < len(s); i++ {
		d[i] = s[i].(string)
	}
	return d
}

func startLambda(handler http.Handler) error {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	lh := &httpAdapterHandler{
		http: handler,
	}
	lambda.StartHandler(lh)
	return nil
}

func ListenAndServe(addr string, handler http.Handler) error {
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		return startLambda(handler)
	} else {
		return http.ListenAndServe(addr, handler)
	}
}
