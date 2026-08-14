#include "message_converter.h"
#include <windows.h>
#include <cctype>
#include <string>

namespace go_mapi {
namespace message_converter {

std::string WideToUtf8(const wchar_t* wide) {
    if (!wide || !wide[0]) return "";
    int size = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
    if (size <= 0) return "";
    std::string result(size - 1, 0);
    WideCharToMultiByte(CP_UTF8, 0, wide, -1, &result[0], size, NULL, NULL);
    return result;
}

std::string AnsiToUtf8(const char* ansi) {
    if (!ansi || !ansi[0]) return "";
    // Step 1: ANSI (system codepage) → UTF-16
    int wideLen = MultiByteToWideChar(CP_ACP, 0, ansi, -1, NULL, 0);
    if (wideLen <= 0) return ansi;  // fallback: return raw bytes
    std::wstring wide(wideLen - 1, 0);
    MultiByteToWideChar(CP_ACP, 0, ansi, -1, &wide[0], wideLen);
    // Step 2: UTF-16 → UTF-8
    return WideToUtf8(wide.c_str());
}

std::string FilenameFromPath(const std::string& path) {
    // Extract filename from a Windows path (backslash or forward slash)
    auto pos = path.find_last_of("\\/");
    if (pos != std::string::npos && pos + 1 < path.size()) {
        return path.substr(pos + 1);
    }
    return path;
}

// ARRICKS-02 helper: reserved DOS device names are rejected by the filesystem
// regardless of extension, so "CON", "con.pdf" and "NUL.txt" all fail.
static bool IsReservedDeviceName(const std::string& name) {
    // Compare only the stem, upper-cased.
    std::string stem = name.substr(0, name.find('.'));
    for (auto& c : stem) {
        c = static_cast<char>(::toupper(static_cast<unsigned char>(c)));
    }
    static const char* kReserved[] = {
        "CON", "PRN", "AUX", "NUL",
        "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
        "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
    };
    for (const char* r : kReserved) {
        if (stem == r) return true;
    }
    return false;
}

std::string SanitizeFilename(const std::string& nameOrPath) {
    // Always reduce to a leaf name first. FilenameFromPath deliberately
    // returns its input unchanged when the string ends in a separator (that
    // behaviour is locked in by message_converter_tests.cpp), so strip
    // separators here rather than relying on it. This also neutralises
    // "..\..\" traversal and any drive-qualified path.
    std::string name = nameOrPath;
    auto pos = name.find_last_of("\\/");
    if (pos != std::string::npos) {
        name = name.substr(pos + 1);
    }

    // Replace everything NTFS rejects, plus control characters. Note ':' also
    // closes off NTFS alternate data streams ("report.pdf:hidden").
    static const std::string kIllegal = "<>:\"/\\|?*";
    std::string out;
    out.reserve(name.size());
    for (unsigned char c : name) {
        if (c < 0x20 || c == 0x7F ||
            kIllegal.find(static_cast<char>(c)) != std::string::npos) {
            out += '_';
        } else {
            out += static_cast<char>(c);
        }
    }

    // Windows silently drops trailing dots and spaces, which would leave the
    // recorded name and the name on disk disagreeing. Leading dots are legal
    // and preserved; leading spaces are not useful and are dropped.
    while (!out.empty() && (out.back() == '.' || out.back() == ' ')) {
        out.pop_back();
    }
    size_t firstKeep = out.find_first_not_of(' ');
    if (firstKeep == std::string::npos) {
        out.clear();
    } else if (firstKeep > 0) {
        out = out.substr(firstKeep);
    }

    // Cap the length so destDir + basename stays clear of MAX_PATH, keeping
    // the extension so the attachment still opens with the right application.
    if (out.size() > kMaxSanitizedBasename) {
        std::string ext;
        auto dot = out.find_last_of('.');
        if (dot != std::string::npos && dot > 0 && out.size() - dot <= 12) {
            ext = out.substr(dot);
        }
        size_t keep = kMaxSanitizedBasename > ext.size()
                          ? kMaxSanitizedBasename - ext.size()
                          : 0;
        // Never cut a multi-byte UTF-8 sequence in half — the result is
        // serialised into JSON and would corrupt the queue file.
        while (keep > 0 &&
               (static_cast<unsigned char>(out[keep]) & 0xC0) == 0x80) {
            --keep;
        }
        out = out.substr(0, keep) + ext;
    }

    if (out.empty() || out == "." || out == ".." || IsReservedDeviceName(out)) {
        // "attachment" with no extension is deliberate: we have nothing
        // trustworthy left to infer one from. The Gmail-side MIME filename
        // still carries the caller's original name.
        return "attachment";
    }
    return out;
}

// QUICK-260423-qpx: many legacy Simple MAPI callers (Spanish SendEmail-style
// apps, older accounting software, Win32 utilities that predate the modern
// lpszName/lpszAddress split) populate only lpszName with a bare email and
// leave lpszAddress NULL. Promote when the name looks like an email so the
// Go validator can accept the message. Only considers '@' as the signal --
// the Go side (normalizeAddress) handles further cleanup.
static void PromoteEmailShapedNameToAddress(Recipient& r) {
    if (!r.address.empty()) return;
    if (r.name.empty()) return;
    if (r.name.find('@') == std::string::npos) return;
    r.address = r.name;
    r.name.clear();
}

MailMessage ConvertAnsiMessage(const MapiMessage& msg) {
    MailMessage result;
    // originApp is populated by the DLL glue layer (MapiImpl::GetOriginApplicationName),
    // which requires a live process context and stays outside this pure module.
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
            PromoteEmailShapedNameToAddress(r);

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

MailMessage ConvertWideMessage(const MapiMessageW& msg) {
    MailMessage result;
    // originApp is populated by the DLL glue layer (MapiImpl::GetOriginApplicationName),
    // which requires a live process context and stays outside this pure module.
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
            PromoteEmailShapedNameToAddress(r);

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

} // namespace message_converter
} // namespace go_mapi
