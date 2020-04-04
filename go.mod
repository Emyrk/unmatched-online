module github.com/Emyrk/unmatched-online

go 1.13

require (
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/websocket v1.4.1
	github.com/pschlump/godebug v1.0.1 // indirect
	github.com/pschlump/json v1.12.0 // indirect
	github.com/pschlump/socketio v0.0.0-20180411001808-3a9d761e99b5
	github.com/rs/cors v1.7.0
	github.com/sirupsen/logrus v1.5.0
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/stretchr/testify v1.4.0 // indirect
	gopkg.in/yaml.v2 v2.2.4 // indirect
)

replace github.com/pschlump/socketio => github.com/Emyrk/socketio v0.0.0-20200330184531-5eb3ae5686db
