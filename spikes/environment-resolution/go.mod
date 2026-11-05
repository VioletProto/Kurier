module github.com/VioletProto/Kurier/spikes/environment-resolution

go 1.26.0

require github.com/VioletProto/Kurier/services/request-runtime v0.0.0-00010101000000-000000000000

require (
	github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2
	github.com/getkin/kin-openapi v0.152.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/VioletProto/Kurier/services/request-runtime => ../../services/request-runtime
