/* mapiprobe.c — empirical verifier for ARRICKS-10 dual-bitness MAPI
 * registration.
 *
 * Calls MAPISendMail through the in-box mapi32.dll stub exactly the way real
 * Simple MAPI callers (32-bit scanner software, 64-bit Explorer "Send to")
 * do, then reports which go-mapi.dll the stub actually resolved and loaded.
 * This is the ground truth the registry assertions in installer.Tests.ps1
 * cannot provide: it proves the stub expands the REG_EXPAND_SZ DLLPath with
 * the *caller's* environment, per bitness, on real Windows.
 *
 * Built twice (x64 + x86) by installer-smoke.yml with the same clang
 * drivers as the interceptor; asserted by installer.Tests.ps1 items 28/29.
 *
 * Output contract (one KEY=value per line, parsed by Pester):
 *   BITNESS=64|32
 *   PROGRAMFILES=<per-process %ProgramFiles% expansion>
 *   MAPIRC=<MAPISendMail return code>       (0 = SUCCESS_SUCCESS)
 *   RESOLVED=<full path of loaded go-mapi.dll>|<none>
 *
 * Exit codes: 0 = go-mapi.dll loaded in-process; 2 = stub missing/broken;
 * 3 = stub ran but never loaded go-mapi.dll (misregistration).
 */

#include <windows.h>
#include <stdio.h>

/* Simple MAPI ABI — mirrors src/interceptor/mapi_types.h (ANSI variant). */
typedef ULONG_PTR LHANDLE;
typedef ULONG FLAGS;

typedef struct {
    ULONG ulReserved;
    ULONG flFlags;
    ULONG nPosition;
    LPSTR lpszPathName;
    LPSTR lpszFileName;
    LPVOID lpFileType;
} MapiFileDesc, *LPMapiFileDesc;

typedef struct {
    ULONG ulReserved;
    ULONG ulRecipClass;
    LPSTR lpszName;
    LPSTR lpszAddress;
    ULONG ulEIDSize;
    LPVOID lpEntryID;
} MapiRecipDesc, *LPMapiRecipDesc;

typedef struct {
    ULONG ulReserved;
    LPSTR lpszSubject;
    LPSTR lpszNoteText;
    LPSTR lpszMessageType;
    LPSTR lpszDateReceived;
    LPSTR lpszConversationID;
    FLAGS flFlags;
    LPMapiRecipDesc lpOriginator;
    ULONG nRecipCount;
    LPMapiRecipDesc lpRecips;
    ULONG nFileCount;
    LPMapiFileDesc lpFiles;
} MapiMessage, *LPMapiMessage;

#define MAPI_TO 1

typedef ULONG (WINAPI *PFN_MAPISendMail)(LHANDLE, ULONG_PTR, LPMapiMessage,
                                         FLAGS, ULONG);

int main(void) {
    char programFiles[MAX_PATH] = "";
    ExpandEnvironmentStringsA("%ProgramFiles%", programFiles,
                              sizeof programFiles);
    printf("BITNESS=%u\n", (unsigned)(sizeof(void *) * 8));
    printf("PROGRAMFILES=%s\n", programFiles);

    HMODULE stub = LoadLibraryA("mapi32.dll");
    if (stub == NULL) {
        printf("MAPIRC=-1\nRESOLVED=<none>\n");
        fprintf(stderr, "LoadLibrary(mapi32.dll) failed, gle=%lu\n",
                GetLastError());
        return 2;
    }
    PFN_MAPISendMail sendMail =
        (PFN_MAPISendMail)GetProcAddress(stub, "MAPISendMail");
    if (sendMail == NULL) {
        printf("MAPIRC=-1\nRESOLVED=<none>\n");
        fprintf(stderr, "GetProcAddress(MAPISendMail) failed, gle=%lu\n",
                GetLastError());
        return 2;
    }

    MapiRecipDesc recip = {0};
    recip.ulRecipClass = MAPI_TO;
    recip.lpszName = "smoke probe";
    recip.lpszAddress = "SMTP:probe@example.invalid";

    MapiMessage msg = {0};
    msg.lpszSubject = "arricks installer-smoke mapiprobe";
    msg.lpszNoteText = "dual-bitness DLLPath resolution probe";
    msg.nRecipCount = 1;
    msg.lpRecips = &recip;

    /* flFlags 0: no dialog, no logon UI — the scanner-software code path. */
    ULONG rc = sendMail(0, 0, &msg, 0, 0);
    printf("MAPIRC=%lu\n", rc);

    HMODULE provider = GetModuleHandleA("go-mapi.dll");
    if (provider == NULL) {
        printf("RESOLVED=<none>\n");
        return 3;
    }
    char resolved[MAX_PATH] = "";
    GetModuleFileNameA(provider, resolved, sizeof resolved);
    printf("RESOLVED=%s\n", resolved);
    return 0;
}
