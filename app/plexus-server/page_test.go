package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	router *gin.Engine
)

func TestMain(m *testing.M) {
	os.Setenv("DB_FILE", "test.db")
	//_ := initializeDatabase()
	//defer Close()
	//checkDefaultUser()
	router = setupRouter()
	os.Exit(m.Run())
}

func TestDisplayMainPage(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(body), "<title>Plexus</title>")
}

func TestGetPage(t *testing.T) {
	// no user
	page := getPage("someone")
	assert.Equal(t, "v0.1.0", page.Version)
}

func TestSetTheme(t *testing.T) {
	SetTheme("themeuser", "black")
	page := getPage("themeuser")
	assert.Equal(t, "black", page.Theme)
}

func TestSetFont(t *testing.T) {
	SetFont("fontuser", "Lato")
	page := getPage("fontuser")
	assert.Equal(t, "Lato", page.Font)
}

func TestSetRefresh(t *testing.T) {
	SetRefresh("refreshuser", 2)
	page := getPage("refreshuser")
	assert.Equal(t, 2, page.Refresh)
}
