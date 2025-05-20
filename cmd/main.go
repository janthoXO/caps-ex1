package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
)

// Defines a "model" that we can use to communicate with the
// frontend or the database
// More on these "tags" like `bson:"_id,omitempty"`: https://go.dev/wiki/Well-known-struct-tags
type BookStore struct {
	MongoID     primitive.ObjectID `bson:"_id,omitempty"`
	ID          string             `bson:"id,omitempty"`
	BookName    string             `bson:"bookname,omitempty"`
	BookAuthor  string             `bson:"bookauthor,omitempty"`
	BookEdition string             `bson:"bookedition,omitempty"`
	BookPages   string             `bson:"bookpages,omitempty"`
	BookYear    string             `bson:"bookyear,omitempty"`
}

type BookDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Pages   string `json:"pages"`
	Edition string `json:"edition"`
	Year    string `json:"year"`
}

func (b BookStore) ToDTO() BookDTO {
	return BookDTO{
		ID:      b.ID,
		Title:   b.BookName,
		Author:  b.BookAuthor,
		Pages:   b.BookPages,
		Edition: b.BookEdition,
		Year:    b.BookYear,
	}
}

func (b *BookStore) FromDTO(dto BookDTO) {
	b.ID = dto.ID
	b.BookName = dto.Title
	b.BookAuthor = dto.Author
	b.BookEdition = dto.Edition
	b.BookPages = dto.Pages
	b.BookYear = dto.Year
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

// Here we make sure the connection to the database is correct and initial
// configurations exists. Otherwise, we create the proper database and collection
// we will store the data.
// To ensure correct management of the collection, we create a return a
// reference to the collection to always be used. Make sure if you create other
// files, that you pass the proper value to ensure communication with the
// database
// More on what bson means: https://www.mongodb.com/docs/drivers/go/current/fundamentals/bson/
func prepareDatabase(client *mongo.Client, dbName string, collecName string) (*mongo.Collection, error) {
	db := client.Database(dbName)

	names, err := db.ListCollectionNames(context.TODO(), bson.D{{}})
	if err != nil {
		return nil, err
	}

	log.Debugf("Collections in database %s: %v", dbName, names)
	if !slices.Contains(names, collecName) {
		cmd := bson.D{{"create", collecName}}
		var result bson.M
		if err = db.RunCommand(context.TODO(), cmd).Decode(&result); err != nil {
			log.Fatal(err)
			return nil, err
		}
	}

	coll := db.Collection(collecName)

	// Create a unique index on the "id" field
	_, err = coll.Indexes().CreateOne(
		context.TODO(),
		mongo.IndexModel{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// Create a composite unique index on name, author, year and pages
    _, err = coll.Indexes().CreateOne(
        context.TODO(),
        mongo.IndexModel{
            Keys: bson.D{
                {Key: "bookname", Value: 1},
                {Key: "bookauthor", Value: 1},
                {Key: "bookyear", Value: 1},
                {Key: "bookpages", Value: 1},
            },
            Options: options.Index().SetUnique(true),
        },
    )
    if err != nil {
        log.Fatal(err)
        return nil, err
    }

	return coll, nil
}

// Here we prepare some fictional data and we insert it into the database
// the first time we connect to it. Otherwise, we check if it already exists.
func prepareData(client *mongo.Client, coll *mongo.Collection) {
	startData := []BookStore{
		{
			ID:          "example1",
			BookName:    "The Vortex",
			BookAuthor:  "José Eustasio Rivera",
			BookEdition: "958-30-0804-4",
			BookPages:   "292",
			BookYear:    "1924",
		},
		{
			ID:          "example2",
			BookName:    "Frankenstein",
			BookAuthor:  "Mary Shelley",
			BookEdition: "978-3-649-64609-9",
			BookPages:   "280",
			BookYear:    "1818",
		},
		{
			ID:          "example3",
			BookName:    "The Black Cat",
			BookAuthor:  "Edgar Allan Poe",
			BookEdition: "978-3-99168-238-7",
			BookPages:   "280",
			BookYear:    "1843",
		},
	}

	// This syntax helps us iterate over arrays. It behaves similar to Python
	// However, range always returns a tuple: (idx, elem). You can ignore the idx
	// by using _.
	// In the topic of function returns: sadly, there is no standard on return types from function. Most functions
	// return a tuple with (res, err), but this is not granted. Some functions
	// might return a ret value that includes res and the err, others might have
	// an out parameter.
	for _, book := range startData {
		cursor, err := coll.Find(context.TODO(), book)
		var results []BookStore
		if err = cursor.All(context.TODO(), &results); err != nil {
			panic(err)
		}
		if len(results) > 1 {
			log.Fatal("more records were found")
		} else if len(results) == 0 {
			result, err := coll.InsertOne(context.TODO(), book)
			if err != nil {
				panic(err)
			} else {
				fmt.Printf("%+v\n", result)
			}

		} else {
			for _, res := range results {
				cursor.Decode(&res)
				fmt.Printf("%+v\n", res)
			}
		}
	}
}

func insertBook(coll *mongo.Collection, book BookStore) (BookStore, error) {
	result, err := coll.InsertOne(context.TODO(), book)
	if err != nil {
		return BookStore{}, err
	}

	book.MongoID = result.InsertedID.(primitive.ObjectID)
	return book, nil
}

// Generic method to perform "SELECT * FROM BOOKS" (if this was SQL, which
// it is not :D ), and then we convert it into an array of map. In Golang, you
// define a map by writing map[<key type>]<value type>{<key>:<value>}.
// interface{} is a special type in Golang, basically a wildcard...
func findAllBooks(coll *mongo.Collection) ([]BookStore, error) {
	cursor, err := coll.Find(context.TODO(), bson.D{{}})
	if err != nil {
		return nil, err
	}

	var results []BookStore
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func updateBook(coll *mongo.Collection, id string, book BookStore) (BookStore, error) {
	filter := bson.M{"id": id}
	update := bson.M{"$set": book}

	result, err := coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return BookStore{}, err
	}

	log.Debugf("result: %+v", result)
	if result.ModifiedCount == 0 && result.MatchedCount == 0 {
		return BookStore{}, fmt.Errorf("no book found with id %s", id)
	}

	return book, nil
}

func deleteBook(coll *mongo.Collection, id string) error {
	_, err := coll.DeleteOne(context.TODO(), bson.M{"id": id})
	return err
}

func findAllAuthors(coll *mongo.Collection) (authors []string, err error) {
	var results []interface{}
	results, err = coll.Distinct(context.TODO(), "bookauthor", bson.D{{}})
	if err != nil {
		return nil, err
	}

	for _, res := range results {
		authors = append(authors, res.(string))
	}

	return authors, err
}

func findAllYears(coll *mongo.Collection) (years []string, err error) {
	var results []interface{}
	results, err = coll.Distinct(context.TODO(), "bookyear", bson.D{{}})
	if err != nil {
		return nil, err
	}

	for _, res := range results {
		years = append(years, res.(string))
	}

	return years, err
}

func main() {
	// Connect to the database. Such defer keywords are used once the local
	// context returns; for this case, the local context is the main function
	// By user defer function, we make sure we don't leave connections
	// dangling despite the program crashing. Isn't this nice? :D
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get MongoDB URI from environment variable or use default
	LoadConfig()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(Cfg.Database.Url))

	// This is another way to specify the call of a function. You can define inline
	// functions (or anonymous functions, similar to the behavior in Python)
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	// You can use such name for the database and collection, or come up with
	// one by yourself!
	coll, err := prepareDatabase(client, Cfg.Database.Name, "information")
	if err != nil {
		log.Fatal(err)
	}

	prepareData(client, coll)

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
		books, err := findAllBooks(coll)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		var ret []map[string]interface{}
		for _, res := range books {
			ret = append(ret, map[string]interface{}{
				"ID":          res.MongoID.Hex(),
				"BookName":    res.BookName,
				"BookAuthor":  res.BookAuthor,
				"BookEdition": res.BookEdition,
				"BookPages":   res.BookPages,
			})
		}

		return c.Render(200, "book-table", ret)
	})

	e.GET("/authors", func(c echo.Context) error {
		authors, err := findAllAuthors(coll)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		return c.Render(200, "author-list", authors)
	})

	e.GET("/years", func(c echo.Context) error {
		years, err := findAllYears(coll)
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

	// You will have to expand on the allowed methods for the path
	// `/api/route`, following the common standard.
	// A very good documentation is found here:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Methods
	// It specifies the expected returned codes for each type of request
	// method.
	e.GET("/api/books", func(c echo.Context) error {
		books, err := findAllBooks(coll)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		var ret []BookDTO
		for _, book := range books {
			bookDTO := book.ToDTO()
			ret = append(ret, bookDTO)
		}
		return c.JSON(http.StatusOK, ret)
	})

	e.POST("/api/books", func(c echo.Context) error {
		var err error
		book := BookDTO{}
		if err = c.Bind(&book); err != nil {
			log.Error(err)
			return c.NoContent(http.StatusBadRequest)
		}
		if book.ID == "" || book.Title == "" || book.Author == "" {
			log.Error("Missing required fields")
			return c.NoContent(http.StatusBadRequest)
		}

		// Convert DTO to BookStore
		bookStore := BookStore{}
		bookStore.FromDTO(book)

		// Insert the book into the database
		bookStore, err = insertBook(coll, bookStore)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		BookDTO := bookStore.ToDTO()
		return c.JSON(http.StatusCreated, BookDTO)
	})

	e.PUT("/api/books/:id", func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			log.Error("Missing ID")
			return c.NoContent(http.StatusBadRequest)
		}
		var err error
		book := BookDTO{}
		if err = c.Bind(&book); err != nil {
			log.Error(err)
			return c.NoContent(http.StatusBadRequest)
		}
		book.ID = id
		bookStore := BookStore{}
		bookStore.FromDTO(book)

		bookStore, err = updateBook(coll, id, bookStore)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		return c.NoContent(http.StatusOK)
	})

	e.DELETE("/api/books/:id", func(c echo.Context) error {
		id := c.Param("id")
		if id == "" {
			log.Error("Missing ID")
			return c.NoContent(http.StatusBadRequest)
		}
		err := deleteBook(coll, id)
		if err != nil {
			log.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		return c.NoContent(http.StatusOK)
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
