package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHelloWorldHandler(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/helloworld", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if assert.NoError(t, HelloWorldHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "Hello, World!", rec.Body.String())
	}
}
