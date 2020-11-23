# Custom URL Shortner

Requires Go to be installed to compile

## Install
```bash
$ go build
```
```bash
$ cp config-example.json config.json
```
```bash
$ ./go-shortener
```

## Config
Server is used for application settings, like port or app name.
Database is used to configure database settings
Domains are all domains currently and previously available to create new urls for, if the domain is not allowed to create new urls, use `false`
Defaultdomain is used when a short url is requested to a domain not in the domains list or to a domain set to `false`
Users.anonymous controls if anonymous users can create shortlinks, if false, users need an API key

```json
{
    "server": {
        "port": ":8080"
    },
    "database": {
        "type" : "mysql",
        "user" : "root",
        "password": "",
        "host": "localhost",
        "port": "3306",
        "name": "go-shortener"
    },
    "domains": {
        "https://example.com/":    true
    },
    "defaultDomain": "https://example.com/",
    "users": {
        "anonymous" : false
    }
}
```

## Endpoints
Create short
```bash
POST    /new/basic

{
    "url": "https://example.com/longurl",
    "custom": "customshort",
    "domain" "https://domain.com/"
}
```

Use short link
```bash
GET     /:url
```

## Database

Currently only MYSQL is supported, see go-shortener.sql for structure
