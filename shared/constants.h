/**
 * Shared constants and types for go-mapi
 */

#pragma once

// Version and constants
#define GO_MAPI_VERSION_MAJOR 0
#define GO_MAPI_VERSION_MINOR 1
#define GO_MAPI_VERSION_PATCH 0

// JSON schema version for message format
#define GO_MAPI_JSON_VERSION 1

// Default directories
#ifdef _WIN32
#define GO_MAPI_DEFAULT_WATCH_DIR "%TEMP%\\go-mapi"
#define GO_MAPI_PROCESSED_SUBDIR "processed"
#define GO_MAPI_ERRORS_SUBDIR "errors"
#endif

// Message format identifier
#define GO_MAPI_MESSAGE_PREFIX "msg_"
#define GO_MAPI_MESSAGE_SUFFIX ".json"

// Application metadata
#define GO_MAPI_APP_NAME "go-mapi"
#define GO_MAPI_DLL_NAME "go-mapi.dll"
