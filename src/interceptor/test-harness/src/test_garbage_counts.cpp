// ARRICKS-12 (R7): garbage nRecipCount / nFileCount must be rejected before
// any element is dereferenced. A count of 100000 with a one-element array is
// the uninitialized-struct case that used to walk lpRecips off the end of
// real memory — the caps turn it into a clean error return, and the SEH
// guard (where the toolchain has it) backstops anything the caps can't see.

#include <windows.h>
#include <iostream>
#include "../test_utils.h"

using namespace mapi_test;

// Local mirror of the Simple MAPI error codes under test (mapi_types.h).
static constexpr ULONG kTooManyFiles = 9;      // MAPI_E_TOO_MANY_FILES
static constexpr ULONG kTooManyRecipients = 10; // MAPI_E_TOO_MANY_RECIPIENTS

int test_garbage_counts() {
    std::cout << "\nTest: Garbage recipient/file counts (R7 caps)" << std::endl;

    HMODULE hDll = LoadLibraryA("DraftHorse.dll");
    if (!hDll) {
        std::cerr << "Failed to load DraftHorse.dll" << std::endl;
        return 1;
    }

    MAPISendMailFunc MAPISendMail = reinterpret_cast<MAPISendMailFunc>(
        GetProcAddress(hDll, "MAPISendMail")
    );
    if (!MAPISendMail) {
        std::cerr << "Failed to get MAPISendMail function" << std::endl;
        FreeLibrary(hDll);
        return 1;
    }

    char toAddress[] = "test@example.com";
    MapiRecipDesc recipient = {};
    recipient.ulRecipClass = MAPI_TO;
    recipient.lpszAddress = toAddress;

    int failures = 0;

    {
        MapiMessage message = {};
        message.nRecipCount = 100000; // garbage — array has ONE element
        message.lpRecips = &recipient;
        ULONG result = MAPISendMail(0, 0, &message, 0, 0);
        std::cout << "nRecipCount=100000 returned: " << result << std::endl;
        if (result != kTooManyRecipients) {
            std::cerr << "FAIL: expected MAPI_E_TOO_MANY_RECIPIENTS ("
                      << kTooManyRecipients << ")" << std::endl;
            ++failures;
        }
    }

    {
        MapiMessage message = {};
        message.nRecipCount = 1;
        message.lpRecips = &recipient;
        message.nFileCount = 100000; // garbage — no file array at all
        message.lpFiles = nullptr;
        ULONG result = MAPISendMail(0, 0, &message, 0, 0);
        std::cout << "nFileCount=100000 returned: " << result << std::endl;
        if (result != kTooManyFiles) {
            std::cerr << "FAIL: expected MAPI_E_TOO_MANY_FILES ("
                      << kTooManyFiles << ")" << std::endl;
            ++failures;
        }
    }

    FreeLibrary(hDll);
    return failures == 0 ? 0 : 1;
}
