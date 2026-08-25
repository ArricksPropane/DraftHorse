#include "mapi_impl.h"
#include "message_converter.h"
#include "fs_utils.h"
#include "json_writer.h"
#include <windows.h>
#include <psapi.h>
#include <cstdint>
#include <string>
#include <vector>

#pragma comment(lib, "psapi.lib")

namespace go_mapi {

// ARRICKS-04: sanity bound on a caller-supplied attachment list. Gmail rejects
// anything remotely near this, so a larger list means the caller handed us
// garbage rather than a real message.
static constexpr size_t kMaxAttachmentCount = 256;

// ARRICKS-12 (R7): same reasoning for recipients. A count beyond this is an
// uninitialized or hostile MapiMessage, not mail — and iterating a garbage
// nRecipCount walks lpRecips off the end of real memory. Checked BEFORE any
// element is dereferenced.
static constexpr ULONG kMaxRecipientCount = 500;

// QUICK-260423-tk6: copy attachments into a stable sibling dir keyed off the
// supplied stem. On success, mutates msg.attachments in-place so each entry's
// `path` points at the new copy and `size` reflects the copied byte count.
// On any failure, best-effort removes partial files and writes the reason to
// errors\<stem>.error so the Wails app surfaces it. Returns true iff every
// attachment landed (or there were none).
//
// Rationale: the legacy Spanish MAPI app deletes its own TEMP dir as soon as
// MAPISendMail returns, so if we leave the original paths in the JSON the
// Wails app later hits "attachment not found" on draft creation.
static bool CopyAttachmentsForStem(MailMessage& msg, const std::wstring& stem) {
    if (msg.attachments.empty()) return true;

    std::wstring attachDir = FsUtils::GetAttachmentsDirForStem(stem);
    if (attachDir.empty()) return false;
    if (!FsUtils::EnsureDirExists(attachDir)) {
        FsUtils::WriteErrorForStem(stem,
            "failed to create attachments directory");
        return false;
    }

    std::vector<std::wstring> landed;  // for rollback on partial failure
    for (auto& att : msg.attachments) {
        if (att.path.empty()) {
            // No path means nothing to copy — leave entry as-is (Gmail side
            // will skip empty-path attachments). Still counts as success.
            continue;
        }
        // ARRICKS-02: prefer the caller's explicit filename, fall back to the
        // path, then sanitise unconditionally.
        //
        // The previous code passed the raw value straight through to
        // CopyFileToDir. Two bugs followed. First, message_converter guards on
        // the lpszFileName *pointer* rather than on emptiness, so a non-NULL
        // but empty lpszFileName (common in older MFC wrappers) left
        // att.filename empty and this fell back to the full path — producing
        // a destination of "...\queue\<stem>\C:\TEMP\scan.pdf", which fails to
        // copy and drops the whole message. Second, nothing rejected NTFS
        // -illegal characters or "..\" traversal.
        //
        // SanitizeFilename reduces a path to its leaf, so both cases collapse
        // to the same safe answer. att.filename is deliberately left alone:
        // it becomes the MIME filename in the draft, where the caller's
        // original name is both valid and friendlier.
        std::string basename = message_converter::SanitizeFilename(
            !att.filename.empty() ? att.filename : att.path);

        std::wstring newPath;
        uint32_t newSize = 0;
        if (!FsUtils::CopyFileToDir(att.path, attachDir, basename,
                                    newPath, newSize)) {
            // Roll back any files we already copied this message to keep the
            // queue clean — half-a-message would be worse than nothing.
            for (const auto& p : landed) {
                DeleteFileW(p.c_str());
            }
            RemoveDirectoryW(attachDir.c_str());
            FsUtils::WriteErrorForStem(stem,
                "failed to copy attachment to queue");
            return false;
        }
        landed.push_back(newPath);

        // Rewrite attachment path/size so the Gmail client reads from the
        // stable copy instead of the caller's about-to-be-deleted TEMP.
        int n = WideCharToMultiByte(CP_UTF8, 0, newPath.c_str(), -1,
                                    nullptr, 0, nullptr, nullptr);
        if (n > 0) {
            std::string newPathUtf8(n - 1, 0);
            WideCharToMultiByte(CP_UTF8, 0, newPath.c_str(), -1,
                                &newPathUtf8[0], n, nullptr, nullptr);
            att.path = newPathUtf8;
        }
        att.size = newSize;
    }
    return true;
}

// ARRICKS-03: shared tail for every entry point that queues a message.
// Generates the stem, copies attachments into the queue-owned sibling dir,
// then writes the JSON.
//
// Also fixes the leak on the JSON-write failure path: the attachment-copy
// failure path had careful rollback but this one had none, so a failed write
// (disk full, AV lock, stem collision) left copies of the attachments in
// %LOCALAPPDATA% with nothing to ever clean them up.
static ULONG QueueMessage(MailMessage& msg) {
    std::wstring stem = FsUtils::GenerateUniqueStem();
    if (!CopyAttachmentsForStem(msg, stem)) {
        // Error file already written by CopyAttachmentsForStem; do NOT write
        // the JSON — half a message would silently drop attachments.
        return MAPI_E_FAILURE;
    }

    std::wstring filePath = JsonWriter::WriteMailToFileWithStem(msg, stem);
    if (filePath.empty()) {
        FsUtils::RemoveAttachmentsDirForStem(stem);
        FsUtils::WriteErrorForStem(stem, "failed to write queue JSON");
        return MAPI_E_FAILURE;
    }
    return SUCCESS_SUCCESS;
}

// ARRICKS-04 helper: split a delimited list as supplied to MAPISendDocuments.
// lpszDelimChar may name more than one delimiter character; any of them
// separates entries. Surrounding whitespace is trimmed and empty entries are
// dropped, since "a; b;" is common in the wild.
static std::vector<std::string> SplitDelimited(const std::string& s,
                                               const std::string& delims) {
    std::vector<std::string> out;
    if (s.empty()) return out;

    const std::string effective = delims.empty() ? std::string(";") : delims;
    size_t start = 0;
    while (start <= s.size()) {
        size_t pos = s.find_first_of(effective, start);
        std::string piece = (pos == std::string::npos)
                                ? s.substr(start)
                                : s.substr(start, pos - start);

        size_t b = piece.find_first_not_of(" \t");
        size_t e = piece.find_last_not_of(" \t");
        if (b != std::string::npos) {
            out.push_back(piece.substr(b, e - b + 1));
        }

        if (pos == std::string::npos) break;
        start = pos + 1;
    }
    return out;
}

std::string MapiImpl::GetOriginApplicationName() {
    wchar_t processPath[MAX_PATH];
    HANDLE hProcess = GetCurrentProcess();

    if (GetModuleFileNameExW(hProcess, nullptr, processPath, MAX_PATH)) {
        // Get just the filename from the full path
        wchar_t* filename = wcsrchr(processPath, L'\\');
        if (filename) {
            filename++;  // Move past the backslash
        } else {
            filename = processPath;
        }

        // Convert to UTF-8
        int size_needed = WideCharToMultiByte(CP_UTF8, 0, filename, -1, NULL, 0, NULL, NULL);
        std::string result(size_needed - 1, 0);
        WideCharToMultiByte(CP_UTF8, 0, filename, -1, &result[0], size_needed, NULL, NULL);
        return result;
    }

    return "unknown.exe";
}

// ARRICKS-12 (R7): the *Body functions hold all C++ objects; the *Guarded
// wrappers hold only the SEH frame. The split is mandatory — a function
// cannot mix __try with objects needing unwinding (MSVC C2712; clang agrees).
// An access violation from a garbage MapiMessage pointer graph is NOT a C++
// exception, so the catch (...) below never sees it; only SEH does. The
// guard requires BOTH __SEH__ (SEH unwinding on this target) and __clang__
// (GCC defines __SEH__ on x86_64 but has no __try keyword at all; clang has
// it behind -fms-extensions, which CMakeLists.txt sets for this file).
// Where the guard compiles out, the caps above the call are the remaining
// protection and the body runs unwrapped, as it always did.
// On AV inside the body, its objects are abandoned without unwinding —
// leaking a string is the right trade against corrupting the host app.

static ULONG SendMailABody(const MapiMessage& message, const std::string& originApp) {
    try {
        MailMessage msg = message_converter::ConvertAnsiMessage(message);
        msg.originApp = originApp;

        // QUICK-260423-tk6: copy attachments into a queue-owned sibling dir
        // BEFORE writing the JSON. The legacy Spanish MAPI caller deletes its
        // own TEMP directory as soon as this function returns, so the Wails
        // app would otherwise see "attachment not found" on draft creation.
        return QueueMessage(msg);
    } catch (...) {
        return MAPI_E_FAILURE;
    }
}

static ULONG SendMailWBody(const MapiMessageW& message, const std::string& originApp) {
    try {
        MailMessage msg = message_converter::ConvertWideMessage(message);
        msg.originApp = originApp;

        // QUICK-260423-tk6: same lifetime fix as the ANSI path — copy
        // attachments into %LOCALAPPDATA%\DraftHorse\queue\<stem>\ before the
        // caller's TEMP dir disappears on return.
        return QueueMessage(msg);
    } catch (...) {
        return MAPI_E_FAILURE;
    }
}

static ULONG SendMailAGuarded(LPMapiMessage lpMessage, const std::string& originApp) {
#if defined(__SEH__) && defined(__clang__)
    __try {
        return SendMailABody(*lpMessage, originApp);
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return MAPI_E_FAILURE;
    }
#else
    return SendMailABody(*lpMessage, originApp);
#endif
}

static ULONG SendMailWGuarded(LPMapiMessageW lpMessage, const std::string& originApp) {
#if defined(__SEH__) && defined(__clang__)
    __try {
        return SendMailWBody(*lpMessage, originApp);
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return MAPI_E_FAILURE;
    }
#else
    return SendMailWBody(*lpMessage, originApp);
#endif
}

ULONG MapiImpl::MAPISendMailA(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    LPMapiMessage lpMessage,
    FLAGS flFlags,
    ULONG ulReserved
) {
    if (!lpMessage) {
        return MAPI_E_INVALID_MESSAGE;
    }
    // ARRICKS-12 (R7): reject garbage counts before any element access.
    if (lpMessage->nRecipCount > kMaxRecipientCount) {
        return MAPI_E_TOO_MANY_RECIPIENTS;
    }
    if (lpMessage->nFileCount > kMaxAttachmentCount) {
        return MAPI_E_TOO_MANY_FILES;
    }
    // originApp resolved out here: it needs the private member (live process
    // context), and it must not construct inside the SEH frame below.
    return SendMailAGuarded(lpMessage, GetOriginApplicationName());
}

ULONG MapiImpl::MAPISendMailW(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    LPMapiMessageW lpMessage,
    FLAGS flFlags,
    ULONG ulReserved
) {
    if (!lpMessage) {
        return MAPI_E_INVALID_MESSAGE;
    }
    // ARRICKS-12 (R7): reject garbage counts before any element access.
    if (lpMessage->nRecipCount > kMaxRecipientCount) {
        return MAPI_E_TOO_MANY_RECIPIENTS;
    }
    if (lpMessage->nFileCount > kMaxAttachmentCount) {
        return MAPI_E_TOO_MANY_FILES;
    }
    return SendMailWGuarded(lpMessage, GetOriginApplicationName());
}

ULONG MapiImpl::MAPILogon(
    ULONG_PTR ulUIParam,
    LPSTR lpszProfileName,
    LPSTR lpszPassword,
    FLAGS flFlags,
    ULONG ulReserved,
    LPLHANDLE lphSession
) {
    // Stub: just return success
    if (lphSession) {
        *lphSession = 1;  // Return a dummy session handle
    }
    return SUCCESS_SUCCESS;
}

ULONG MapiImpl::MAPILogoff(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    FLAGS flFlags,
    ULONG ulReserved
) {
    // Stub: just return success
    return SUCCESS_SUCCESS;
}

ULONG MapiImpl::MAPIFreeBuffer(LPVOID pv) {
    // Stub: nothing to free in our implementation
    return SUCCESS_SUCCESS;
}

// ARRICKS-04: MAPISendDocuments was exported but was a stub that returned
// SUCCESS_SUCCESS without doing anything. Callers were told the mail had been
// handled, typically deleted their temp file, and nothing was ever queued —
// silent data loss with a positive confirmation to the user.
//
// MAPISendDocuments is the simplest Simple MAPI entry point and is a common
// choice for scanner / MFP "Scan to Email" software and older line-of-business
// applications, which is precisely the population this deployment serves.
//
// Semantics: lpszFilePaths is a delimited list of full paths, lpszFileNames an
// optional parallel list of display names, and lpszDelimChar names the
// delimiter (semicolon when absent). There are no recipients, subject or body
// — the user supplies those in the draft.
// ARRICKS-12 (R7): same body/guard split as MAPISendMail — AnsiToUtf8 walks
// caller-supplied C strings, so a non-terminated buffer can AV mid-scan.
static ULONG SendDocumentsBody(
    LPSTR lpszDelimChar,
    LPSTR lpszFilePaths,
    LPSTR lpszFileNames,
    const std::string& originApp
) {
    try {
        // Convert first, then split. Splitting the raw ANSI bytes risks
        // cutting a lead-byte pair on a DBCS system code page; ASCII
        // delimiters are unambiguous once the text is UTF-8.
        const std::string pathsUtf8 = message_converter::AnsiToUtf8(lpszFilePaths);
        const std::string namesUtf8 =
            lpszFileNames ? message_converter::AnsiToUtf8(lpszFileNames) : std::string();
        const std::string delims =
            (lpszDelimChar && lpszDelimChar[0])
                ? message_converter::AnsiToUtf8(lpszDelimChar)
                : std::string(";");

        const std::vector<std::string> paths = SplitDelimited(pathsUtf8, delims);
        const std::vector<std::string> names = SplitDelimited(namesUtf8, delims);

        if (paths.empty()) {
            return MAPI_E_ATTACHMENT_NOT_FOUND;
        }
        if (paths.size() > kMaxAttachmentCount) {
            return MAPI_E_TOO_MANY_FILES;
        }

        MailMessage msg;
        msg.bodyFormat = "plain";
        msg.originApp = originApp;

        for (size_t i = 0; i < paths.size(); ++i) {
            Attachment att;
            att.path = paths[i];
            // The display-name list is optional and may be shorter than the
            // path list; fall back to the leaf of the path.
            att.filename = (i < names.size() && !names[i].empty())
                               ? names[i]
                               : message_converter::FilenameFromPath(paths[i]);
            att.size = 0;
            msg.attachments.push_back(att);
        }

        return QueueMessage(msg);
    } catch (...) {
        return MAPI_E_FAILURE;
    }
}

static ULONG SendDocumentsGuarded(
    LPSTR lpszDelimChar,
    LPSTR lpszFilePaths,
    LPSTR lpszFileNames,
    const std::string& originApp
) {
#if defined(__SEH__) && defined(__clang__)
    __try {
        return SendDocumentsBody(lpszDelimChar, lpszFilePaths, lpszFileNames, originApp);
    } __except (EXCEPTION_EXECUTE_HANDLER) {
        return MAPI_E_FAILURE;
    }
#else
    return SendDocumentsBody(lpszDelimChar, lpszFilePaths, lpszFileNames, originApp);
#endif
}

ULONG MapiImpl::MAPISendDocuments(
    ULONG_PTR ulUIParam,
    LPSTR lpszDelimChar,
    LPSTR lpszFilePaths,
    LPSTR lpszFileNames,
    ULONG ulReserved
) {
    if (!lpszFilePaths || !lpszFilePaths[0]) {
        return MAPI_E_ATTACHMENT_NOT_FOUND;
    }
    return SendDocumentsGuarded(lpszDelimChar, lpszFilePaths, lpszFileNames,
                                GetOriginApplicationName());
}

} // namespace go_mapi
