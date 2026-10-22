module github.com/VioletProto/Kurier/spikes/environment-resolution

go 1.26.0

require github.com/VioletProto/Kurier/services/request-runtime v0.0.0-00010101000000-000000000000

require github.com/cyberphone/json-canonicalization v0.0.0-20261011152517-32a48195b9b2 // indirect

replace github.com/VioletProto/Kurier/services/request-runtime => ../../services/request-runtime
