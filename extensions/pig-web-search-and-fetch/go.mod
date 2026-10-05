module github.com/xinaps-dev/pig-web-search-and-fetch/extensions/pig-web-search-and-fetch

go 1.26

// The SDK is pinned to a published release so this module builds anywhere. PiG
// stages its own copy of the SDK when it builds the extension into a runtime
// cell, so the pin never blocks an install.
//
// Keep this in step with the PiG release you target.
require github.com/MichaelKinsy/PiG/extensions/sdk v0.4.0
