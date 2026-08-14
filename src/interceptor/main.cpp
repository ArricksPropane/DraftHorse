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
    LPMapiMessageW lpMessage,
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
//
// ARRICKS-01: DllMain runs under the Windows loader lock. It must not touch
// the filesystem, the registry, or shell APIs, and must not allocate in a way
// that can throw — see "Dynamic-Link Library Best Practices" (Microsoft).
//
// The previous implementation called FsUtils::EnsureOutputDirectory() here,
// which reaches SHGetFolderPathW + SHCreateDirectoryExW. Shell path resolution
// dynamically loads further DLLs and reads the registry, all under the loader
// lock, and the std::wstring it builds can throw bad_alloc straight into the
// loader. This DLL is loaded in-process by every application that touches
// Simple MAPI (including explorer.exe), so a deadlock here hangs the host
// application at load time with no useful diagnostic.
//
// The call was also redundant: JsonWriter::WriteMailToFile and
// WriteMailToFileWithStem both call EnsureOutputDirectory() at the point the
// directory is actually needed (json_writer.cpp:124,150), which is the correct
// place for it.
BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call, LPVOID lpReserved) {
    switch (ul_reason_for_call) {
    case DLL_PROCESS_ATTACH:
        // Nothing to initialize. Queue directories are created lazily on first
        // write. Opting out of thread notifications also avoids needless
        // loader callbacks in thread-heavy host applications.
        DisableThreadLibraryCalls(hModule);
        break;
    case DLL_PROCESS_DETACH:
    case DLL_THREAD_ATTACH:
    case DLL_THREAD_DETACH:
        break;
    }
    return TRUE;
}
