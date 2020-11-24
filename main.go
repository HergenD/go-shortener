package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ilyakaznacheev/cleanenv"
)

type ConfigServer struct {
	Port   string `json:"port" env:"SV_PORT" env-default:"5432"`
	Host   string `json:"host" env:"SV_HOST" env-default:"localhost"`
	Scheme string `json:"scheme" env:"SV_SCHEME" env-default:"https"`
	Base   string `json:"base" env:"SV_BASE" env-default:"/"`
	Name   string `json:"name" env:"SV_NAME" env-default:"Go Shortener"`
}

type ConfigDatabase struct {
	Type     string `json:"type" env:"DB_TYPE" env-default:"mysql"`
	User     string `json:"user" env:"DB_USER" env-default:"root"`
	Password string `json:"password" env:"DB_PASSWORD" env-default:""`
	Host     string `json:"host" env:"DB_HOST" env-default:"localhost"`
	Port     string `json:"port" env:"DB_PORT" env-default:"3306"`
	Name     string `json:"name" env:"DB_NAME" env-default:"go-shortener"`
}

type ConfigUsers struct {
	Anonymous bool `json:"anonymous" env:"ANONYMOUS" env-default:"false"`
}

type Config struct {
	Server        ConfigServer    `json:"server"`
	Database      ConfigDatabase  `json:"database"`
	Users         ConfigUsers     `json:"users"`
	Domains       map[string]bool `json:"domains"`
	DefaultDomain string          `json:"defaultDomain" env:"DOMAIN_DEFAULT" env-default:"https://example.com/"`
	Logfile       string          `json:"logFile" env:"LOG_FILE" env-default:"shortener.log"`
}

type BasicUrl struct {
	Url    string `form:"url" json:"url" binding:"required"`
	Domain string `form:"domain" json:"domain"`
	Custom string `form:"custom" json:"custom"`
}

type Entry struct {
	Id     int
	Short  string
	Long   LongUrl
	Domain string
	User   User
}

type LongUrl struct {
	Full     string
	Scheme   string
	Host     string
	Port     string
	Path     string
	Fragment string
	Query    string
}

type User struct {
	Id       int
	Username string
	ApiKey   string
}

var cfg Config
var databases = map[string]map[string]string{}
var database_type string
var database_connection string

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(location.Default())

	r.Use(location.New(location.Config{
		Scheme:  cfg.Server.Scheme,
		Host:    cfg.Server.Host,
		Base:    cfg.Server.Base,
		Headers: location.Headers{Scheme: "X-Forwarded-Proto", Host: "X-Forwarded-Host"},
	}))

	//Routes
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "https://github.com/HergenD/go-shortener")
	})
	r.GET("/:url", getUrl)
	// r.POST("/get/all", getAll)
	r.POST("/create", postUrl)

	return r
}

func getUrl(c *gin.Context) {
	origin := location.Get(c)
	shortUrl := c.Param("url")
	baseDomain := origin.Scheme + "://" + origin.Host + "/"

	// Allows localhost origin to act as default domain for dev purposes
	if origin.Host == "localhost"+cfg.Server.Port {
		baseDomain = cfg.DefaultDomain
	}

	longUrl, ok := databases[baseDomain][shortUrl]

	if ok {
		c.Redirect(http.StatusMovedPermanently, longUrl)
	} else {
		c.JSON(http.StatusOK, gin.H{"short": shortUrl, "long": false})
	}
}

func getAll(c *gin.Context) {
	c.JSON(http.StatusOK, databases)
}

func getUser(bearer string) User {
	const BEARER_SCHEMA = "Bearer "
	tokenString := bearer[len(BEARER_SCHEMA):]
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var user User
	user_sql := "SELECT * FROM users WHERE `api_key`='" + tokenString + "'"
	user_row := db.QueryRow(user_sql)
	user_row.Scan(&user.Id, &user.Username, &user.ApiKey)

	return user
}

func postUrl(c *gin.Context) {
	var entry Entry
	if c.GetHeader("Authorization") != "" {
		entry.User = getUser(c.GetHeader("Authorization"))
	}
	if entry.User.Id == 0 && cfg.Users.Anonymous == false {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "please provide api key"})
		return
	}
	// Bind JSON from request to variable and set some initials variables
	origin := location.Get(c)
	var json BasicUrl
	if c.BindJSON(&json) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"required": "url"})
		return
	}
	longUrl := json.Url

	var baseDomain string
	if json.Domain != "" && cfg.Domains[json.Domain] {
		baseDomain = json.Domain
	} else if cfg.Domains[origin.Scheme+"://"+origin.Host+"/"] {
		baseDomain = origin.Scheme + "://" + origin.Host + "/"
	} else {
		baseDomain = cfg.DefaultDomain
	}

	entry.Long = parseLong(longUrl)
	entry.Domain = baseDomain

	if json.Custom == "" {
		entry = createRandomShort(entry)
	} else {
		entry = createCustomShort(json.Custom, entry)
	}
	if entry.Short == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "short url could not be generated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"short": entry.Domain + entry.Short, "long": entry.Long.Full})
	return
}

func parseLong(long string) LongUrl {
	var l LongUrl

	u, err := url.Parse(long)
	if err != nil {
		log.Fatal(err)
	}
	if u.Scheme == "" {
		long = "https://" + long
		u, err = url.Parse(long)
		if err != nil {
			log.Fatal(err)
		}
	}
	l.Scheme = u.Scheme
	l.Full = long
	l.Host = u.Host
	host, port, _ := net.SplitHostPort(u.Host)
	if host != "" && port != "" {
		l.Host = host
		l.Port = port
	}

	l.Path = u.Path
	l.Fragment = u.Fragment
	l.Query = u.RawQuery

	return l
}

func createCustomShort(custom string, entry Entry) Entry {
	// Connect to database
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, found := databases[entry.Domain][custom]
	if found {
		return entry
	}

	entry.Short = custom
	// Create full short url based on domain and update memory database with new short
	databases[entry.Domain][entry.Short] = entry.Long.Full

	// Update MYSQL database wih new shortener
	sql := "INSERT INTO entries" +
		"(`Short`, `Long`, `Domain`, `LongScheme`, `LongHost`, `LongPort`, `LongPath`, `LongFragment`, `LongQuery`, `User`)" +
		" VALUES " +
		"('" + entry.Short + "', '" + entry.Long.Full + "', '" + entry.Domain + "', '" +
		entry.Long.Scheme + "', '" + entry.Long.Host + "', '" + entry.Long.Port + "', '" + entry.Long.Path + "', '" +
		entry.Long.Fragment + "', '" + entry.Long.Query + "', '" + strconv.Itoa(entry.User.Id) + "')"

	_, err = db.Exec(sql)
	if err != nil {
		log.Fatal(err)
	}

	return entry
}

func createRandomShort(entry Entry) Entry {
	// Connect to database
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check if url has been shortened using the given domain
	// if true, give previously created shortlink
	var exists bool
	exists_sql := "SELECT EXISTS(SELECT 1 FROM entries WHERE `Long`='" + entry.Long.Full + "' AND `Domain`='" + entry.Domain + "' AND `User`='" + strconv.Itoa(entry.User.Id) + "')"
	row := db.QueryRow(exists_sql)
	if err := row.Scan(&exists); err != nil {
		log.Fatal(err)
	} else if exists {
		entry_sql := "SELECT `Id`, `Long`, `Short`, `Domain` FROM entries WHERE `Long`='" + entry.Long.Full + "' AND `Domain`='" + entry.Domain + "' AND `User`='" + strconv.Itoa(entry.User.Id) + "'  LIMIT 1"
		entry_row := db.QueryRow(entry_sql)
		entry_row.Scan(&entry.Id, &entry.Long.Full, &entry.Short, &entry.Domain)
		return entry
	}

	found := true
	var short string

	// Create short urlpart, and check if it exists, if it does, generate new short (loop)
	for found {
		short = createShort(6)
		_, found = databases[entry.Domain][short]
	}

	entry.Short = short

	// Create full short url based on domain and update memory database with new short
	databases[entry.Domain][entry.Short] = entry.Long.Full
	// Update MYSQL database wih new shortener
	sql := "INSERT INTO entries" +
		"(`Short`, `Long`, `Domain`, `LongScheme`, `LongHost`, `LongPort`, `LongPath`, `LongFragment`, `LongQuery`, `User`)" +
		" VALUES " +
		"('" + entry.Short + "', '" + entry.Long.Full + "', '" + entry.Domain + "', '" +
		entry.Long.Scheme + "', '" + entry.Long.Host + "', '" + entry.Long.Port + "', '" + entry.Long.Path + "', '" +
		entry.Long.Fragment + "', '" + entry.Long.Query + "', '" + strconv.Itoa(entry.User.Id) + "')"
	_, err = db.Exec(sql)

	if err != nil {
		log.Fatal(err)
	}

	return entry
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

func main() {
	// Read config file
	err := cleanenv.ReadConfig("config.json", &cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.Users.Anonymous)
	// Set logging to logfile
	file, err := os.OpenFile(cfg.Logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
	log.Println("Starting " + cfg.Server.Name)

	// Start application
	fmt.Println(color.BlueString("Starting"), cfg.Server.Name)

	// Set database settings from config
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

	// Connect to database
	fmt.Println(color.CyanString("Connecting"), "to database...")
	db, err := sql.Open(database_type, database_connection)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check database version
	var version string
	err2 := db.QueryRow("SELECT VERSION()").Scan(&version)
	if err2 != nil {
		log.Fatal(err2)
	}
	fmt.Println(color.GreenString("Connected."), "Running version "+version)

	// Count number of entries to be imported
	var count int
	count_sql := "SELECT COUNT(Id) FROM entries;"
	row := db.QueryRow(count_sql)
	if err := row.Scan(&count); err != nil {
		log.Fatal(err)
	}
	count_string := strconv.Itoa(count)
	fmt.Println(color.CyanString("Importing"), count_string+" entries from database...")

	// Query all entries from database
	res, err := db.Query("SELECT `Id`, `Long`, `Short`, `Domain` FROM entries")
	defer res.Close()
	if err != nil {
		log.Fatal(err)
	}

	// Create memory database (map) based on imported records
	for domain := range cfg.Domains {
		databases[domain] = map[string]string{}
	}
	for res.Next() {
		var entry Entry
		err := res.Scan(&entry.Id, &entry.Long.Full, &entry.Short, &entry.Domain)

		if err != nil {
			log.Fatal(err)
		}

		databases[entry.Domain][entry.Short] = entry.Long.Full
	}
	fmt.Println(color.GreenString("Successfully imported"), count_string+" entries from database.")

	// Set up router and finish booting
	fmt.Println(color.CyanString("Starting"), "router...")
	r := setupRouter()
	r.Run(cfg.Server.Port)
}
