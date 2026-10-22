module github.com/VioletProto/Kurier/spikes/openapi-library-selection

go 1.26.0

replace github.com/VioletProto/Kurier/services/request-runtime => ../../services/request-runtime

require (
	github.com/VioletProto/Kurier/services/request-runtime v0.0.0-00010101000000-000000000000
	github.com/getkin/kin-openapi v0.152.0
	github.com/oasdiff/yaml3 v0.0.14
	github.com/pb33f/libopenapi v0.41.6
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/pb33f/go-yaml v0.1.1 // indirect
	github.com/pb33f/jsonpath v0.8.4 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.2 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
