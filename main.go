package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var domains = []string{
	"https://365.works/",
	"https://365werk.nl/",
}

var databases = map[string]map[string]string{}

type BasicUrl struct {
	Url string `form:"url" json:"url" binding:"required"`
}

func setupRouter() *gin.Engine {

	r := gin.Default()

	//Routes
	r.GET("/:url", getUrl)
	r.POST("/get/all", getAll)
	r.POST("/new/basic", postBasicUrl)

	return r
}

func getUrl(c *gin.Context) {
	shortUrl := c.Param("url")
	baseDomain := domains[0]
	longUrl, ok := databases[baseDomain][shortUrl]
	if ok {
		c.Redirect(http.StatusMovedPermanently, longUrl)
		// c.JSON(http.StatusOK, gin.H{"short": shortUrl, "long": longUrl})
	} else {
		c.JSON(http.StatusOK, gin.H{"short": shortUrl, "long": false})
	}
}

func getAll(c *gin.Context) {
	c.JSON(http.StatusOK, databases)
}

func postBasicUrl(c *gin.Context) {
	var json BasicUrl
	if c.BindJSON(&json) == nil {
		longUrl := json.Url
		baseDomain := domains[0]

		short := createShort(6)
		found := true
		var shortUrl string
		for found {
			shortUrl, found = databases[baseDomain][short]
		}
		shortUrl = baseDomain + short
		databases[baseDomain][short] = longUrl

		db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/go-shortener")
		if err != nil {
			panic(err)
		}
		sql := "INSERT INTO entries(`Short`, `Long`, `Domain`) VALUES ('" + short + "', '" + longUrl + "', '" + baseDomain + "')"
		fmt.Println(sql)
		_, err = db.Exec(sql)
		defer db.Close()

		if err != nil {
			panic(err.Error())
		}

		c.JSON(http.StatusOK, gin.H{"short": shortUrl, "long": longUrl})
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func createShort(length int) string {
	return StringWithCharset(length, charset)
}

type Entries struct {
	Id     int
	Short  string
	Long   string
	Domain string
}

func main() {
	fmt.Println("Connecting to database...")
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/go-shortener")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var version string

	err2 := db.QueryRow("SELECT VERSION()").Scan(&version)

	if err2 != nil {
		panic(err2)
	}

	fmt.Println(version)

	res, err := db.Query("SELECT * FROM entries")

	defer res.Close()

	if err != nil {
		panic(err)
	}

	for _, domain := range domains {
		databases[domain] = map[string]string{}
	}

	for res.Next() {
		var entry Entries
		err := res.Scan(&entry.Id, &entry.Long, &entry.Short, &entry.Domain)

		if err != nil {
			log.Fatal(err)
		}

		databases[entry.Domain][entry.Short] = entry.Long
	}

	fmt.Println("Starting Router...")
	r := setupRouter()
	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")
}
