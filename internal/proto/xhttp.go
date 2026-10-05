package proto

// Wire settings supported by both mihomo and Xray. Unknown options are rejected,
// rather than silently changing only one end of the connection.
var xhttpExtraKeys = map[string]string{
	"x-padding-bytes": "xPaddingBytes", "x-padding-obfs-mode": "xPaddingObfsMode",
	"x-padding-key": "xPaddingKey", "x-padding-header": "xPaddingHeader",
	"x-padding-placement": "xPaddingPlacement", "x-padding-method": "xPaddingMethod",
	"uplink-http-method": "uplinkHTTPMethod", "session-placement": "sessionPlacement",
	"session-key": "sessionKey", "seq-placement": "seqPlacement", "seq-key": "seqKey",
	"uplink-data-placement": "uplinkDataPlacement", "uplink-data-key": "uplinkDataKey",
	"uplink-chunk-size": "uplinkChunkSize", "no-grpc-header": "noGRPCHeader",
	"sc-max-each-post-bytes": "scMaxEachPostBytes", "sc-min-posts-interval-ms": "scMinPostsIntervalMs",
}
