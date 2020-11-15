package main

import (
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var address string
var storage string
var maxFileSize string
var username string
var password string
var enableAuth bool

// Image type struct
type Image struct {
	Name        string    `json:"name"`
	Time        time.Time `json:"created"`
	Size        int64     `json:"size"`
	IsDirectory bool      `json:"isDirectory"`
}

// Images type struct
type Images struct {
	Image []Image
}

// Error type struct
type Error struct {
	Error string `json:"error"`
}

// Message type struct
type Message struct {
	Message string `json:"message"`
}

// Metadata type struct
type Metadata struct {
	FileName   string `json:"file_name"`
	CreateDate string `json:"create_date"`
	FileSize   int64  `json:"file_size"`
}

func upload(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	var links []string
	files := form.File["files"]

	for _, file := range files {

		id := uuid.New()
		now := time.Now()

		// Source
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		os.MkdirAll(storage, os.ModePerm)

		filename := strings.TrimSuffix(id.String(), "\n")

		dst, err := os.Create(fmt.Sprintf("%s/%s", storage, filename))
		if err != nil {
			return err
		}
		defer dst.Close()

		// Copy
		if _, err = io.Copy(dst, src); err != nil {
			return err
		}

		meta := Metadata{
			FileName:   file.Filename,
			FileSize:   file.Size,
			CreateDate: now.Format("2006/01/02 15:04:05"),
		}

		metaFile, _ := json.MarshalIndent(meta, "", " ")
		err = ioutil.WriteFile(fmt.Sprintf("%s/%s.json", storage, filename), metaFile, 0644)
		if err != nil {
			return err
		}

		// Generate json metadata
		links = append(links, fmt.Sprintf("%s/%s", storage, filename))
	}

	return c.JSON(http.StatusOK, links)
}

func list(c echo.Context) error {

	files, err := ioutil.ReadDir(storage)
	if err != nil {
		return err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})

	images := []Image{}

	for _, f := range files {

		images = append(images, Image{Name: f.Name(), Time: f.ModTime(), Size: f.Size(), IsDirectory: f.IsDir()})
	}

	return c.JSON(http.StatusOK, images)
}

func main() {
	flag.StringVar(&address, "address", ":9090", "Listen address")
	flag.StringVar(&storage, "storage", "media", "Where to store images")
	flag.StringVar(&maxFileSize, "maxFileSize", "50M", "Max upload size")
	flag.StringVar(&username, "username", "user", "BasicAuth user to protect upload")
	flag.StringVar(&password, "password", "pass", "BasicAuth password to protect upload")
	flag.BoolVar(&enableAuth, "enableAuth", false, "Use BasicAuth to protect upload")
	flag.Parse()
	e := echo.New()

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(maxFileSize))
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup: "header:X-CSRF-Token",
	}))
	g := e.Group("/api")
	if enableAuth {
		g.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			// Be careful to use constant time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(username), []byte(username)) == 1 &&
				subtle.ConstantTimeCompare([]byte(password), []byte(password)) == 1 {
				return true, nil
			}
			return false, nil
		}))
	}

	e.Use(middleware.Secure())

	e.Static("/", "public")
	e.Static("/media", storage)
	e.GET("/", func(c echo.Context) (err error) {
		pusher, ok := c.Response().Writer.(http.Pusher)
		if ok {
			if err = pusher.Push("/app.css", nil); err != nil {
				return
			}
			if err = pusher.Push("/app.js", nil); err != nil {
				return
			}
		}
		return c.File("public/index.html")
	})
	g.POST("/upload", upload)
	g.GET("/list", list)

	e.GET("/request", func(c echo.Context) error {
		req := c.Request()
		format := `
			<code>
			Protocol: %s<br>
			Host: %s<br>
			Remote Address: %s<br>
			Method: %s<br>
			Path: %s<br>
			</code>
		`
		return c.HTML(http.StatusOK, fmt.Sprintf(format, req.Proto, req.Host, req.RemoteAddr, req.Method, req.URL.Path))
	})
	e.Logger.Fatal(e.Start(address))
}
