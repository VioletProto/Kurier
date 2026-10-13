module github.com/VioletProto/Kurier/services/local-agent

go 1.26.0

require (
	github.com/VioletProto/Kurier/services/request-runtime v0.0.0
	golang.org/x/term v0.39.0
)

require (
	github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2 // indirect
	golang.org/x/sys v0.40.0 // indirect
)

replace github.com/VioletProto/Kurier/services/request-runtime => ../request-runtime
