// Package schema embeds the JSON Schema of the collector's detailed
// artifact, its methods for the access artifact, and its trace map. Each
// artifact is checked against its schema when it is emitted. Every pack's
// build file carries the schemas and the trace map.
package schema

import (
	_ "embed"
	"encoding/json"
)

// Monitoring is the schema in sentry.monitoring.v1.0.0.json.
//
//go:embed sentry.monitoring.v1.0.0.json
var Monitoring []byte

// TraceMap is the map in trace-map.json. It links each field of the artifact
// to the calls and code behind it, and `make trace-map` rewrites it.
//
//go:embed trace-map.json
var TraceMap []byte

// methods holds methods.json. It has one plain sentence on how the collector
// measures each field of the evidencepack/access-posture@v1 artifact.
//
//go:embed methods.json
var methods []byte

// Methods returns the sentences in methods.json for
// componentsdk.CollectorSpec.Methods. The SDK writes each one as the
// x-epack-method of its field in its own evidencepack/access-posture@v1
// schema.
func Methods() map[string]map[string]string {
	var byField map[string]string
	if err := json.Unmarshal(methods, &byField); err != nil {
		panic("docs/schema/methods.json: " + err.Error())
	}
	return map[string]map[string]string{"evidencepack/access-posture@v1": byField}
}
