package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ilyakaznacheev/cleanenv"
)

type ConfigServer struct {
	Port string `json:"port" env:"PORT" env-default:"5432"`
	Host string `json:"host" env:"HOST" env-default:"localhost"`
	Name string `json:"name" env:"NAME" env-default:"Go Shortener"`
}

type ConfigDatabase struct {
	Type     string `json:"type" env:"TYPE" env-default:"mysql"`
	User     string `json:"user" env:"USER" env-default:"root"`
	Password string `json:"password" env:"PASSWORD" env-default:""`
	Host     string `json:"host" env:"HOST" env-default:"localhost"`
	Port     string `json:"port" env:"PORT" env-default:"3306"`
	Name     string `json:"name" env:"NAME" env-default:"go-shortener"`
}

type Config struct {
	Server        ConfigServer    `json:"server"`
	Database      ConfigDatabase  `json:"database"`
	Domains       map[string]bool `json:"domains"`
	DefaultDomain string          `json:"defaultDomain" env:"DEFAULTDOMAIN" env-default:"https://365.works/"`
}

var cfg Config

var defaultDomain string = "https://365.works/"

var databases = map[string]map[string]string{}

var database_type string
var database_connection string

// Struct for json from POST:/new/basic route
type BasicUrl struct {
	Url    string `form:"url" json:"url" binding:"required"`
	Domain string `form:"domain" json:"domain"`
	Custom string `form:"custom" json:"custom"`
}

func setupRouter() *gin.Engine {

	r := gin.Default()
	r.Use(location.Default())

	r.Use(location.New(location.Config{
		Scheme:  "https",
		Host:    "365.works",
		Base:    "/",
		Headers: location.Headers{Scheme: "X-Forwarded-Proto", Host: "X-Forwarded-Host"},
	}))

	//Routes
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "https://365werk.nl")
	})
	r.GET("/:url", getUrl)
	r.POST("/get/all", getAll)
	r.POST("/new/basic", postBasicUrl)

	return r
}

func getUrl(c *gin.Context) {
	origin := location.Get(c)
	shortUrl := c.Param("url")
	baseDomain := "https://" + origin.Host + "/"

	// Allows localhost origin to act as default domain for dev purposes
	if origin.Host == "localhost"+cfg.Server.Port {
		baseDomain = defaultDomain
	}

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
	// Bind JSON from request to variable and set some initials variables
	origin := location.Get(c)
	var json BasicUrl
	if c.BindJSON(&json) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"required": "url"})
		return
	}
	longUrl := json.Url
	var baseDomain string
	// baseDomain := json.Domain
	if json.Domain != "" && cfg.Domains[json.Domain] {
		baseDomain = json.Domain
	} else if cfg.Domains[json.Domain] {
		baseDomain = "https://" + origin.Host + "/"
	} else {
		baseDomain = defaultDomain
	}

	if json.Custom == "" {
		c.JSON(http.StatusOK, createRandomShort(baseDomain, longUrl, c))
	} else {
		c.JSON(http.StatusOK, createCustomShort(json.Custom, baseDomain, longUrl, c))
	}

}

func createCustomShort(custom string, baseDomain string, longUrl string, c *gin.Context) gin.H {
	// Connect to database
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	shortUrl, found := databases[baseDomain][custom]

	if found {
		return gin.H{"error": "This custom url already exists", "long": longUrl}
	}

	// Create full short url based on domain and update memory database with new short
	shortUrl = baseDomain + custom
	databases[baseDomain][custom] = longUrl

	// Update MYSQL database wih new shortener
	sql := "INSERT INTO entries(`Short`, `Long`, `Domain`) VALUES ('" + custom + "', '" + longUrl + "', '" + baseDomain + "')"
	_, err = db.Exec(sql)

	if err != nil {
		panic(err.Error())
	}
	return gin.H{"short": shortUrl, "long": longUrl}
}

func createRandomShort(baseDomain string, longUrl string, c *gin.Context) gin.H {
	// Connect to database
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check if url has been shortened using the given domain
	// if true, give previously created shortlink
	var exists bool
	exists_sql := "SELECT EXISTS(SELECT 1 FROM entries WHERE `Long`='" + longUrl + "' AND `Domain`='" + baseDomain + "')"
	row := db.QueryRow(exists_sql)
	if err := row.Scan(&exists); err != nil {
		//
	} else if exists {
		entry_sql := "SELECT * FROM entries WHERE `Long`='" + longUrl + "' AND `Domain`='" + baseDomain + "' LIMIT 1"
		entry_row := db.QueryRow(entry_sql)
		var entry Entries
		entry_row.Scan(&entry.Id, &entry.Long, &entry.Short, &entry.Domain)
		shortUrl := baseDomain + entry.Short
		return gin.H{"short": shortUrl, "long": longUrl}
	}

	found := true
	var shortUrl string
	var short string

	// Create short urlpart, and check if it exists, if it does, generate new short (loop)
	for found {
		short = createShort(6)
		shortUrl, found = databases[baseDomain][short]
	}

	// Create full short url based on domain and update memory database with new short
	shortUrl = baseDomain + short
	databases[baseDomain][short] = longUrl

	// Update MYSQL database wih new shortener
	sql := "INSERT INTO entries(`Short`, `Long`, `Domain`) VALUES ('" + short + "', '" + longUrl + "', '" + baseDomain + "')"
	_, err = db.Exec(sql)

	if err != nil {
		panic(err.Error())
	}
	return gin.H{"short": shortUrl, "long": longUrl}
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

	err := cleanenv.ReadConfig("config.json", &cfg)
	if err != nil {
		//
	}
	fmt.Println(color.BlueString("Starting"), cfg.Server.Name)
	// Set database settings from config and connect
	database_type = cfg.Database.Type
	database_connection = cfg.Database.User +
		":" +
		cfg.Database.Password +
		"@tcp(" +
		cfg.Database.Host +
		":" +
		cfg.Database.Port +
		")/" +
		cfg.Database.Name

	fmt.Println(color.CyanString("Connecting"), "to database...")
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var version string

	err2 := db.QueryRow("SELECT VERSION()").Scan(&version)

	if err2 != nil {
		panic(err2)
	}

	fmt.Println(color.GreenString("Connected."), "Running version "+version)

	var count int
	count_sql := "SELECT COUNT(Id) FROM entries;"
	row := db.QueryRow(count_sql)
	if err := row.Scan(&count); err != nil {
		//
	}
	count_string := strconv.Itoa(count)
	fmt.Println(color.CyanString("Importing"), count_string+" entries from database...")

	res, err := db.Query("SELECT * FROM entries")

	defer res.Close()

	if err != nil {
		panic(err)
	}

	for domain := range cfg.Domains {
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
	fmt.Println(color.GreenString("Successfully imported"), count_string+" entries from database.")
	fmt.Println(color.CyanString("Starting"), "router...")
	r := setupRouter()
	r.Run(cfg.Server.Port)
}
