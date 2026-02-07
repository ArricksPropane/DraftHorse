#pragma once

#include "mapi_types.h"
#include "json_writer.h"

namespace go_mapi {

class MapiImpl {
public:
    // ANSI version of MAPISendMail
    static ULONG MAPISendMailA(
        LHANDLE lhSession,
        ULONG_PTR ulUIParam,
        LPMapiMessage lpMessage,
        FLAGS flFlags,
        ULONG ulReserved
    );

    // Unicode version of MAPISendMail
    static ULONG MAPISendMailW(
        LHANDLE lhSession,
        ULONG_PTR ulUIParam,
        LPMapiMessageW lpMessage,
        FLAGS flFlags,
        ULONG ulReserved
    );

    // Stub implementations
    static ULONG MAPILogon(
        ULONG_PTR ulUIParam,
        LPSTR lpszProfileName,
        LPSTR lpszPassword,
        FLAGS flFlags,
        ULONG ulReserved,
        LPLHANDLE lphSession
    );

    static ULONG MAPILogoff(
        LHANDLE lhSession,
        ULONG_PTR ulUIParam,
        FLAGS flFlags,
        ULONG ulReserved
    );

    static ULONG MAPIFreeBuffer(LPVOID pv);

    static ULONG MAPISendDocuments(
        ULONG_PTR ulUIParam,
        LPSTR lpszDelimChar,
        LPSTR lpszFilePaths,
        LPSTR lpszFileNames,
        ULONG ulReserved
    );

private:
    // Convert ANSI MapiMessage to MailMessage struct
    static MailMessage ConvertAnsiMessage(const MapiMessage& msg);

    // Convert Unicode (wide) MapiMessageW to MailMessage struct
    static MailMessage ConvertWideMessage(const MapiMessageW& msg);

    // Convert wide string (UTF-16) to UTF-8
    static std::string WideToUtf8(const wchar_t* wide);

    // Get application name (for originApp field)
    static std::string GetOriginApplicationName();
};

} // namespace go_mapi
