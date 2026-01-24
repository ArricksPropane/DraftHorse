#include <windows.h>
#include "mapi_impl.h"
#include "mapi_types.h"
#include "fs_utils.h"

// Forward exports - these will be called through the .def file
extern "C" {

ULONG STDAPICALLTYPE MAPISendMail(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    LPMapiMessage lpMessage,
    FLAGS flFlags,
    ULONG ulReserved
) {
    return go_mapi::MapiImpl::MAPISendMailA(lhSession, ulUIParam, lpMessage, flFlags, ulReserved);
}

ULONG STDAPICALLTYPE MAPISendMailW(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    LPMapiMessage lpMessage,
    FLAGS flFlags,
    ULONG ulReserved
) {
    return go_mapi::MapiImpl::MAPISendMailW(lhSession, ulUIParam, lpMessage, flFlags, ulReserved);
}

ULONG STDAPICALLTYPE MAPILogon(
    ULONG_PTR ulUIParam,
    LPSTR lpszProfileName,
    LPSTR lpszPassword,
    FLAGS flFlags,
    ULONG ulReserved,
    LPLHANDLE lphSession
) {
    return go_mapi::MapiImpl::MAPILogon(ulUIParam, lpszProfileName, lpszPassword, flFlags, ulReserved, lphSession);
}

ULONG STDAPICALLTYPE MAPILogoff(
    LHANDLE lhSession,
    ULONG_PTR ulUIParam,
    FLAGS flFlags,
    ULONG ulReserved
) {
    return go_mapi::MapiImpl::MAPILogoff(lhSession, ulUIParam, flFlags, ulReserved);
}

ULONG STDAPICALLTYPE MAPIFreeBuffer(LPVOID pv) {
    return go_mapi::MapiImpl::MAPIFreeBuffer(pv);
}

ULONG STDAPICALLTYPE MAPISendDocuments(
    ULONG_PTR ulUIParam,
    LPSTR lpszDelimChar,
    LPSTR lpszFilePaths,
    LPSTR lpszFileNames,
    ULONG ulReserved
) {
    return go_mapi::MapiImpl::MAPISendDocuments(ulUIParam, lpszDelimChar, lpszFilePaths, lpszFileNames, ulReserved);
}

}  // extern "C"

// DLL Entry Point
BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call, LPVOID lpReserved) {
    switch (ul_reason_for_call) {
    case DLL_PROCESS_ATTACH:
        // Initialize on DLL load
        go_mapi::FsUtils::EnsureOutputDirectory();
        break;
    case DLL_PROCESS_DETACH:
        // Cleanup on DLL unload
        break;
    case DLL_THREAD_ATTACH:
    case DLL_THREAD_DETACH:
        // Thread-specific initialization/cleanup
        break;
    }
    return TRUE;
}
