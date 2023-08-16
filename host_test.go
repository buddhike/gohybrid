package gohybrid

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

type testHandler struct {
	request             *http.Request
	response            []byte
	responseContentType string
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.request = r
	w.Write(h.response)
	if h.responseContentType != "" {
		w.Header().Add("Content-Type", h.responseContentType)
	}
}

type testCase[In any, Out any] struct {
	InputEvent            In
	Path                  string
	Method                string
	RequestBody           string
	QueryStringParameters map[string]string
	Status                int
	ResponseBody          string
	ResponseContentType   string
	OutputEvent           Out
}

func TestAPIGatewayProxyMapping(t *testing.T) {

	cases := []testCase[events.APIGatewayProxyRequest, events.APIGatewayProxyResponse]{
		{
			events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Resource:   "/my/resource",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"",
			events.APIGatewayProxyResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "text/plain; charset=utf-8", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              "world",
				IsBase64Encoded:   false,
			},
		},
		{
			events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Resource:   "/my/resource",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"image/jpeg",
			events.APIGatewayProxyResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "image/jpeg", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              base64.StdEncoding.EncodeToString([]byte("world")),
				IsBase64Encoded:   true,
			},
		},
	}

	for _, c := range cases {
		// Arrange
		r, _ := json.Marshal(c.InputEvent)
		th := &testHandler{
			response:            []byte(c.ResponseBody),
			responseContentType: c.ResponseContentType,
		}
		a := HttpAdapterHandler{
			http: th,
		}

		// Act
		out, err := a.Invoke(context.TODO(), r)
		assert.Nil(t, err)
		assert.Equal(t, c.Path, th.request.URL.Path)
		assert.Equal(t, c.Method, th.request.Method)
		b, _ := io.ReadAll(th.request.Body)
		assert.Equal(t, c.RequestBody, string(b))
		for k, v := range c.QueryStringParameters {
			assert.Equal(t, v, th.request.URL.Query().Get(k))
		}
		var response events.APIGatewayProxyResponse
		json.Unmarshal(out, &response)
		assert.Equal(t, c.OutputEvent, response)
	}
}

func TestALBTargetGroupMapping(t *testing.T) {
	cases := []testCase[events.ALBTargetGroupRequest, events.ALBTargetGroupResponse]{
		{
			events.ALBTargetGroupRequest{
				HTTPMethod: "GET",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"",
			events.ALBTargetGroupResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "text/plain; charset=utf-8", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              "world",
				IsBase64Encoded:   false,
			},
		},
		{
			events.ALBTargetGroupRequest{
				HTTPMethod: "GET",
				Path:       "/my/resource",
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"image/jpeg",
			events.ALBTargetGroupResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "image/jpeg", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              base64.StdEncoding.EncodeToString([]byte("world")),
				IsBase64Encoded:   true,
			},
		},
	}

	for _, c := range cases {
		// Arrange
		r, _ := json.Marshal(c.InputEvent)
		th := &testHandler{
			response:            []byte(c.ResponseBody),
			responseContentType: c.ResponseContentType,
		}
		a := HttpAdapterHandler{
			http: th,
		}

		// Act
		out, err := a.Invoke(context.TODO(), r)
		assert.Nil(t, err)
		assert.Equal(t, c.Path, th.request.URL.Path)
		assert.Equal(t, c.Method, th.request.Method)
		b, _ := io.ReadAll(th.request.Body)
		assert.Equal(t, c.RequestBody, string(b))
		for k, v := range c.QueryStringParameters {
			assert.Equal(t, v, th.request.URL.Query().Get(k))
		}
		var response events.ALBTargetGroupResponse
		json.Unmarshal(out, &response)
		assert.Equal(t, c.OutputEvent, response)
	}
}

func TestAPIGatewayV2HTTPMapping(t *testing.T) {
	cases := []testCase[events.APIGatewayV2HTTPRequest, events.APIGatewayV2HTTPResponse]{
		{
			events.APIGatewayV2HTTPRequest{
				RawPath: "/my/resource",
				RequestContext: events.APIGatewayV2HTTPRequestContext{
					HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
						Method: "GET",
					},
				},
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body: "hello",
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"",
			events.APIGatewayV2HTTPResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "text/plain; charset=utf-8", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              "world",
				IsBase64Encoded:   false,
			},
		},
		{
			events.APIGatewayV2HTTPRequest{
				RawPath: "/my/resource",
				RequestContext: events.APIGatewayV2HTTPRequestContext{
					HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
						Method: "GET",
					},
				},
				QueryStringParameters: map[string]string{
					"a": "b",
				},
				Body:            base64.StdEncoding.EncodeToString([]byte("hello")),
				IsBase64Encoded: true,
			},
			"/my/resource",
			"GET",
			"hello",
			map[string]string{"a": "b"},
			200,
			"world",
			"image/jpeg",
			events.APIGatewayV2HTTPResponse{
				StatusCode:        200,
				Headers:           map[string]string{"Content-Type": "image/jpeg", "Content-Length": "5"},
				MultiValueHeaders: map[string][]string{},
				Body:              base64.StdEncoding.EncodeToString([]byte("world")),
				IsBase64Encoded:   true,
			},
		},
	}

	for _, c := range cases {
		// Arrange
		r, _ := json.Marshal(c.InputEvent)
		th := &testHandler{
			response:            []byte(c.ResponseBody),
			responseContentType: c.ResponseContentType,
		}
		a := HttpAdapterHandler{
			http: th,
		}

		// Act
		out, err := a.Invoke(context.TODO(), r)
		assert.Nil(t, err)
		assert.Equal(t, c.Path, th.request.URL.Path)
		assert.Equal(t, c.Method, th.request.Method)
		b, _ := io.ReadAll(th.request.Body)
		assert.Equal(t, c.RequestBody, string(b))
		for k, v := range c.QueryStringParameters {
			assert.Equal(t, v, th.request.URL.Query().Get(k))
		}
		var response events.APIGatewayV2HTTPResponse
		json.Unmarshal(out, &response)
		assert.Equal(t, c.OutputEvent, response)
	}
}
