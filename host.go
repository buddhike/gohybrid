package gohybrid

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const HeaderIsBase64Encoded = "X-IsBase64Encoded"

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

func (w *bufferedResponse) headers() (map[string]string, map[string][]string) {
	h := make(map[string]string)
	mvh := make(map[string][]string)

	for k, v := range w.header {
		if len(v) == 1 {
			h[k] = v[0]
		} else {
			mvh[k] = v
		}
	}
	return h, mvh
}

type HttpAdapterHandler struct {
	http     http.Handler
	basepath string
}

type HttpAdapterHandlerOption func(h *HttpAdapterHandler)

func WithBasePath(p string) HttpAdapterHandlerOption {
	return func(h *HttpAdapterHandler) {
		if p != "" && !strings.HasPrefix(p, "/") {
			p = fmt.Sprintf("/%s", p)
		}
		h.basepath = p
	}
}

func (h *HttpAdapterHandler) Invoke(ctx context.Context, req []byte) ([]byte, error) {
	m := make(map[string]interface{})
	err := json.Unmarshal(req, &m)
	if err != nil {
		return nil, err
	}
	if rctx, ok := m["requestContext"]; ok {
		k := rctx.(map[string]interface{})
		if _, s := k["elb"]; s {
			return h.handleALBTargetGroupRequest(ctx, m)
		} else if _, s := k["http"]; s {
			return h.handleAPIGatewayV2HttpRequest(ctx, m)
		} else if _, s := k["resourcePath"]; s {
			return h.handleAPIGatewayProxyRequest(ctx, m)
		}
	}
	return nil, errors.New("unsupported integration, supported integrations are: ALB, API Gateway")
}

func (h *HttpAdapterHandler) handleALBTargetGroupRequest(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["path"].(string)
	path = rewritePath(path, h.basepath)
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
	headers, mvheaders := res.headers()
	albres := events.ALBTargetGroupResponse{
		StatusCode:        res.status,
		Headers:           headers,
		MultiValueHeaders: mvheaders,
		Body:              res.buffer.String(),
		IsBase64Encoded:   false,
	}
	return json.Marshal(albres)
}

func (h *HttpAdapterHandler) handleAPIGatewayProxyRequest(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["path"].(string)
	path = rewritePath(path, h.basepath)
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
	headers, mvheaders := res.headers()
	gwres := events.APIGatewayProxyResponse{
		StatusCode:        res.status,
		Headers:           headers,
		MultiValueHeaders: mvheaders,
		Body:              res.buffer.String(),
		IsBase64Encoded:   headers[HeaderIsBase64Encoded] == "1",
	}

	return json.Marshal(gwres)
}

func (h *HttpAdapterHandler) handleAPIGatewayV2HttpRequest(ctx context.Context, event map[string]interface{}) ([]byte, error) {
	path := event["rawPath"].(string)
	path = rewritePath(path, h.basepath)
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
	headers, mvheaders := res.headers()
	gwv2res := events.APIGatewayV2HTTPResponse{
		StatusCode:        res.status,
		Headers:           headers,
		MultiValueHeaders: mvheaders,
		Body:              res.buffer.String(),
		IsBase64Encoded:   false,
	}
	return json.Marshal(gwv2res)
}

func (h *HttpAdapterHandler) extractBodyReader(event map[string]interface{}) io.Reader {
	if body, ok := event["body"].(string); ok {
		if e, ok := event["isBase64Encoded"]; ok {
			if e.(bool) {
				return base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(body))
			}
		}
		return bytes.NewBufferString(body)
	}
	return &bytes.Reader{}
}

func (h *HttpAdapterHandler) mapQueryString(event map[string]interface{}, req *http.Request) {
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
	req.URL.RawQuery = q.Encode()
}

func (h *HttpAdapterHandler) mapHeaders(event map[string]interface{}, req *http.Request) {
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

func rewritePath(path, basepath string) string {
	if strings.HasPrefix(path, basepath) {
		path = strings.Replace(path, basepath, "", 1)
		if path == "" {
			path = "/"
		}
		return path
	}
	return path
}

func toStringSlice(s []interface{}) []string {
	d := make([]string, len(s))
	for i := 0; i < len(s); i++ {
		d[i] = s[i].(string)
	}
	return d
}

func startLambda(handler http.Handler, opts ...HttpAdapterHandlerOption) error {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	lh := &HttpAdapterHandler{
		http: handler,
	}
	for _, opt := range opts {
		opt(lh)
	}
	lambda.StartHandler(lh)
	return nil
}

func ListenAndServe(addr string, handler http.Handler, opts ...HttpAdapterHandlerOption) error {
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		return startLambda(handler, opts...)
	} else {
		return http.ListenAndServe(addr, handler)
	}
}

func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler, opts ...HttpAdapterHandlerOption) error {
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		return startLambda(handler, opts...)
	} else {
		return http.ListenAndServeTLS(addr, certFile, keyFile, handler)
	}
}

func ServerListenAndServe(server *http.Server, opts ...HttpAdapterHandlerOption) error {
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		return startLambda(server.Handler, opts...)
	} else {
		return server.ListenAndServe()
	}
}

func ServerListenAndServeTLS(certFile, keyFile string, server *http.Server, opts ...HttpAdapterHandlerOption) error {
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		return startLambda(server.Handler, opts...)
	} else {
		return server.ListenAndServeTLS(certFile, keyFile)
	}
}
