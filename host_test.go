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
	request  *http.Request
	response []byte
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.request = r
	w.Write(h.response)
}

func TestAPIGatewayProxyRequest(t *testing.T) {
	// Arrange
	e := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Resource:   "/my/resource",
		Path:       "/my/resource",
		QueryStringParameters: map[string]string{
			"a": "b",
		},
		Body: "hello",
	}
	r, _ := json.Marshal(e)
	th := &testHandler{}

	//Act
	a := HttpAdapterHandler{
		http: th,
	}
	res, err := a.Invoke(context.TODO(), r)
	var gwpres events.APIGatewayProxyResponse
	json.Unmarshal(res, &gwpres)

	// Assert
	assert.Equal(t, "/my/resource", th.request.URL.Path)
	assert.Equal(t, "b", th.request.URL.Query().Get("a"))
	assert.Equal(t, "GET", th.request.Method)
	b, _ := io.ReadAll(th.request.Body)
	assert.Equal(t, "hello", string(b))
	assert.Nil(t, err)
	assert.Equal(t, 200, gwpres.StatusCode)
}

func TestAPIGatewayProxyRequestWhenBodyIsBase64Encoded(t *testing.T) {
	// Arrange
	body := base64.StdEncoding.EncodeToString([]byte("hello"))
	e := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Resource:   "/my/resource",
		Path:       "/my/resource",
		QueryStringParameters: map[string]string{
			"a": "b",
		},
		Body:            body,
		IsBase64Encoded: true,
	}
	r, _ := json.Marshal(e)
	th := &testHandler{}

	//Act
	a := HttpAdapterHandler{
		http: th,
	}
	a.Invoke(context.TODO(), r)

	// Assert
	assert.Equal(t, "GET", th.request.Method)
	assert.Equal(t, "/my/resource", th.request.URL.Path)
	assert.Equal(t, "b", th.request.URL.Query().Get("a"))
	b, _ := io.ReadAll(th.request.Body)
	assert.Equal(t, "hello", string(b))
}
