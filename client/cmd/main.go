package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	log "github.com/sirupsen/logrus"
)

type BookDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Pages   string `json:"pages"`
	Edition string `json:"edition"`
	Year    string `json:"year"`
}

// Wraps the "Template" struct to associate a necessary method
// to determine the rendering procedure
type Template struct {
	tmpl *template.Template
}

// Preload the available templates for the view folder.
// This builds a local "database" of all available "blocks"
// to render upon request, i.e., replace the respective
// variable or expression.
// For more on templating, visit https://jinja.palletsprojects.com/en/3.0.x/templates/
// to get to know more about templating
// You can also read Golang's documentation on their templating
// https://pkg.go.dev/text/template
func loadTemplates() *Template {
	return &Template{
		tmpl: template.Must(template.ParseGlob("views/*.html")),
	}
}

// Method definition of the required "Render" to be passed for the Rendering
// engine.
// Contraire to method declaration, such syntax defines methods for a given
// struct. "Interfaces" and "structs" can have methods associated with it.
// The difference lies that interfaces declare methods whether struct only
// implement them, i.e., only define them. Such differentiation is important
// for a compiler to ensure types provide implementations of such methods.
func (t *Template) Render(w io.Writer, name string, data interface{}, ctx echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

func main() {
	LoadConfig()

	// Here we prepare the server
	e := echo.New()

	// Define our custom renderer
	e.Renderer = loadTemplates()

	// Log the requests. Please have a look at echo's documentation on more
	// middleware
	e.Use(middleware.Logger())

	e.Static("/css", "css")

	// Endpoint definition. Here, we divided into two groups: top-level routes
	// starting with /, which usually serve webpages. For our RESTful endpoints,
	// we prefix the route with /api to indicate more information or resources
	// are available under such route.
	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", nil)
	})

	e.GET("/books", func(c echo.Context) error {
		resp, err := http.Get(Cfg.Api.Url + "/api/books")
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		defer resp.Body.Close()

		var books []BookDTO
		err = json.NewDecoder(resp.Body).Decode(&books)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		var ret []map[string]interface{}
		for _, book := range books {
			ret = append(ret, map[string]interface{}{
				"ID":          book.ID,
				"BookName":    book.Title,
				"BookAuthor":  book.Author,
				"BookEdition": book.Edition,
				"BookPages":   book.Pages,
			})
		}

		return c.Render(200, "book-table", ret)
	})

	e.GET("/authors", func(c echo.Context) error {
		resp, err := http.Get(Cfg.Api.Url + "/api/authors")
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		defer resp.Body.Close()

		var authors []string
		err = json.NewDecoder(resp.Body).Decode(&authors)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		return c.Render(200, "author-list", authors)
	})

	e.GET("/years", func(c echo.Context) error {
		resp, err := http.Get(Cfg.Api.Url + "/api/years")
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		defer resp.Body.Close()

		var years []string
		err = json.NewDecoder(resp.Body).Decode(&years)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		return c.Render(200, "year-list", years)
	})

	e.GET("/search", func(c echo.Context) error {
		return c.Render(200, "search-bar", nil)
	})

	e.GET("/create", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	// We start the server and bind it to port 3030. For future references, this
	// is the application's port and not the external one. For this first exercise,
	// they could be the same if you use a Cloud Provider. If you use ngrok or similar,
	// they might differ.
	// In the submission website for this exercise, you will have to provide the internet-reachable
	// endpoint: http://<host>:<external-port>
	log.Infof("Starting server on port %d", Cfg.Server.Port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", Cfg.Server.Port)))
}
