#include "mapi_impl.h"
#include <windows.h>
#include <psapi.h>
#include <string>
#include <vector>

#pragma comment(lib, "psapi.lib")

namespace go_mapi {

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

std::string MapiImpl::WideToUtf8(const wchar_t* wide) {
    if (!wide || !wide[0]) return "";
    int size = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
    if (size <= 0) return "";
    std::string result(size - 1, 0);
    WideCharToMultiByte(CP_UTF8, 0, wide, -1, &result[0], size, NULL, NULL);
    return result;
}

std::string MapiImpl::AnsiToUtf8(const char* ansi) {
    if (!ansi || !ansi[0]) return "";
    // Step 1: ANSI (system codepage) → UTF-16
    int wideLen = MultiByteToWideChar(CP_ACP, 0, ansi, -1, NULL, 0);
    if (wideLen <= 0) return ansi;  // fallback: return raw bytes
    std::wstring wide(wideLen - 1, 0);
    MultiByteToWideChar(CP_ACP, 0, ansi, -1, &wide[0], wideLen);
    // Step 2: UTF-16 → UTF-8
    return WideToUtf8(wide.c_str());
}

std::string MapiImpl::FilenameFromPath(const std::string& path) {
    // Extract filename from a Windows path (backslash or forward slash)
    auto pos = path.find_last_of("\\/");
    if (pos != std::string::npos && pos + 1 < path.size()) {
        return path.substr(pos + 1);
    }
    return path;
}

MailMessage MapiImpl::ConvertAnsiMessage(const MapiMessage& msg) {
    MailMessage result;
    result.originApp = GetOriginApplicationName();
    result.bodyFormat = "plain";

    // Subject — ANSI codepage → UTF-8
    if (msg.lpszSubject) {
        result.subject = AnsiToUtf8(msg.lpszSubject);
    }

    // Body — ANSI codepage → UTF-8
    if (msg.lpszNoteText) {
        result.body = AnsiToUtf8(msg.lpszNoteText);
    }

    // Recipients
    if (msg.lpRecips && msg.nRecipCount > 0) {
        for (ULONG i = 0; i < msg.nRecipCount; ++i) {
            const MapiRecipDesc& recip = msg.lpRecips[i];
            Recipient r;
            if (recip.lpszName) {
                r.name = AnsiToUtf8(recip.lpszName);
            }
            if (recip.lpszAddress) {
                r.address = AnsiToUtf8(recip.lpszAddress);
            }

            switch (recip.ulRecipClass) {
            case MAPI_TO:
                result.toRecipients.push_back(r);
                break;
            case MAPI_CC:
                result.ccRecipients.push_back(r);
                break;
            case MAPI_BCC:
                result.bccRecipients.push_back(r);
                break;
            default:
                result.toRecipients.push_back(r);
                break;
            }
        }
    }

    // Attachments
    if (msg.lpFiles && msg.nFileCount > 0) {
        for (ULONG i = 0; i < msg.nFileCount; ++i) {
            const MapiFileDesc& file = msg.lpFiles[i];
            Attachment attach;
            if (file.lpszPathName) {
                attach.path = AnsiToUtf8(file.lpszPathName);
            }
            if (file.lpszFileName) {
                attach.filename = AnsiToUtf8(file.lpszFileName);
            } else if (!attach.path.empty()) {
                // Windows often leaves lpszFileName NULL — extract from path
                attach.filename = FilenameFromPath(attach.path);
            }
            attach.size = 0;

            result.attachments.push_back(attach);
        }
    }

    return result;
}

MailMessage MapiImpl::ConvertWideMessage(const MapiMessageW& msg) {
    MailMessage result;
    result.originApp = GetOriginApplicationName();
    result.bodyFormat = "plain";

    // Subject
    if (msg.lpszSubject) {
        result.subject = WideToUtf8(msg.lpszSubject);
    }

    // Body
    if (msg.lpszNoteText) {
        result.body = WideToUtf8(msg.lpszNoteText);
    }

    // Recipients
    if (msg.lpRecips && msg.nRecipCount > 0) {
        for (ULONG i = 0; i < msg.nRecipCount; ++i) {
            const MapiRecipDescW& recip = msg.lpRecips[i];
            Recipient r;
            if (recip.lpszName) {
                r.name = WideToUtf8(recip.lpszName);
            }
            if (recip.lpszAddress) {
                r.address = WideToUtf8(recip.lpszAddress);
            }

            switch (recip.ulRecipClass) {
            case MAPI_TO:
                result.toRecipients.push_back(r);
                break;
            case MAPI_CC:
                result.ccRecipients.push_back(r);
                break;
            case MAPI_BCC:
                result.bccRecipients.push_back(r);
                break;
            default:
                result.toRecipients.push_back(r);
                break;
            }
        }
    }

    // Attachments
    if (msg.lpFiles && msg.nFileCount > 0) {
        for (ULONG i = 0; i < msg.nFileCount; ++i) {
            const MapiFileDescW& file = msg.lpFiles[i];
            Attachment attach;
            if (file.lpszPathName) {
                attach.path = WideToUtf8(file.lpszPathName);
            }
            if (file.lpszFileName) {
                attach.filename = WideToUtf8(file.lpszFileName);
            } else if (!attach.path.empty()) {
                // Windows often leaves lpszFileName NULL — extract from path
                attach.filename = FilenameFromPath(attach.path);
            }
            attach.size = 0;

            result.attachments.push_back(attach);
        }
    }

    return result;
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

    try {
        MailMessage msg = ConvertAnsiMessage(*lpMessage);
        std::wstring filePath = JsonWriter::WriteMailToFile(msg);

        if (filePath.empty()) {
            return MAPI_E_FAILURE;
        }

        return SUCCESS_SUCCESS;
    } catch (...) {
        return MAPI_E_FAILURE;
    }
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

    try {
        MailMessage msg = ConvertWideMessage(*lpMessage);
        std::wstring filePath = JsonWriter::WriteMailToFile(msg);

        if (filePath.empty()) {
            return MAPI_E_FAILURE;
        }

        return SUCCESS_SUCCESS;
    } catch (...) {
        return MAPI_E_FAILURE;
    }
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

ULONG MapiImpl::MAPISendDocuments(
    ULONG_PTR ulUIParam,
    LPSTR lpszDelimChar,
    LPSTR lpszFilePaths,
    LPSTR lpszFileNames,
    ULONG ulReserved
) {
    // Stub: not implemented yet
    return SUCCESS_SUCCESS;
}

} // namespace go_mapi
