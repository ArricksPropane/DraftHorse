#pragma once

#include <string>
#include "mapi_types.h"
#include "json_writer.h"

namespace go_mapi {
namespace message_converter {

// Convert an ANSI MAPI message to the internal MailMessage representation.
// Pure conversion — no file I/O, no DLL entry points, no global state.
// Callable from tests via the FOUND-05 OBJECT library target.
MailMessage ConvertAnsiMessage(const MapiMessage& msg);

// Convert a Unicode (wide) MAPI message to the internal MailMessage representation.
// Pure conversion — no file I/O, no DLL entry points, no global state.
MailMessage ConvertWideMessage(const MapiMessageW& msg);

// Convert a wide (UTF-16) C string to UTF-8.
// Returns empty string for nullptr input.
std::string WideToUtf8(const wchar_t* wide);

// Convert an ANSI C string to UTF-8 (passthrough if already ASCII).
// Returns empty string for nullptr input.
std::string AnsiToUtf8(const char* ansi);

// Extract the filename portion of a path (basename).
// Pure string manipulation — handles both forward and backward slashes.
std::string FilenameFromPath(const std::string& path);

// Byte cap applied by SanitizeFilename. Keeps queue paths clear of MAX_PATH:
// %LOCALAPPDATA% + "\DraftHorse\queue\" + 26-char stem + "\" + basename.
constexpr size_t kMaxSanitizedBasename = 128;

// ARRICKS-02: reduce an arbitrary caller-supplied filename (or full path) to a
// leaf name that is safe to create on NTFS.
//
// The caller controls MapiFileDesc::lpszFileName completely, and the previous
// code concatenated it into the destination path unchecked. Two consequences:
//
//  1. Any NTFS-illegal character (: * ? " < > |) made CopyFileW fail, which
//     aborted the entire message with MAPI_E_FAILURE. Scanner software
//     routinely produces names like "Scan 2026-08-14 09:12:33.pdf", so this
//     was a realistic way to lose mail silently.
//  2. A name such as "..\..\evil.lnk" wrote outside the queue directory.
//
// Guarantees about the result: contains no path separators, no characters NTFS
// rejects and no control characters, has no trailing dot or space, is not a
// reserved DOS device name, remains valid UTF-8, is at most
// kMaxSanitizedBasename bytes, and is never empty (falls back to "attachment").
std::string SanitizeFilename(const std::string& nameOrPath);

} // namespace message_converter
} // namespace go_mapi
