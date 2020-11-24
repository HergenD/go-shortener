# Custom URL Shortner
Custom URL shortner that allows you to use your own domain(s) to create short URLs.

The shortner supports creating random short URLs and custom short (or long, depending on your input) urls. You can use any amount of domains with the shortner, a POST to /create makes a new short link of the domain that was posted to, unless otherwise specified in the postdata (`domain`).

The shortner also allows for restricting short creation with api keys, although this is optional.

Todo:
- Creating proper user accounts, there is currently no interface for this. If you want to use API keys, add users with generater keys directly in the database
- Multiple database support / database error handling. The current implementation is not ideal
- Analytics. Currently clicks on shortlinks are not recorded, this is a priority feature.
- Platform detection + alternative long links based on platform. This is to support mobile deeplinking based on mobile platform
- Tests. Currently no tests are in place

Requires:
- Go
- MySQL


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
POST    /create

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
