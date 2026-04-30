module stress-test

go 1.25.2

replace github.com/gzydong/go-chat => ../../

require (
	github.com/gorilla/websocket v1.5.3
	github.com/urfave/cli/v2 v2.27.7
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/text v0.34.0 // indirect
)
