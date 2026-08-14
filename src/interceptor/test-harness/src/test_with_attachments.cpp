#include <windows.h>
#include <cstring>
#include <fstream>
#include <iostream>
#include <string>
#include <filesystem>
#include "../test_utils.h"

using namespace mapi_test;

int test_with_attachments() {
    std::cout << "\nTest: With Attachments" << std::endl;

    // Load the DLL
    HMODULE hDll = LoadLibraryA("go-mapi.dll");
    if (!hDll) {
        std::cerr << "Failed to load go-mapi.dll" << std::endl;
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

    // Create message with attachments
    char subject[] = "Test Email - With Attachments";
    char body[] = "This email has file attachments.";
    char toAddress[] = "test@example.com";
    char toName[] = "Test User";

    // ARRICKS-05: this used a hardcoded C:\test.txt that does not exist on any
    // clean machine. Since quick/260423-tk6 the DLL copies attachments before
    // writing the JSON, so a missing source now fails the copy and returns
    // MAPI_E_FAILURE — this test could not pass. Create a real file instead.
    char filePath[MAX_PATH] = {};
    {
        wchar_t tempPathW[MAX_PATH];
        GetTempPathW(MAX_PATH, tempPathW);
        std::filesystem::path p =
            std::filesystem::path(tempPathW) / "go-mapi-harness-attachment.txt";
        std::string narrow = p.string();
        strncpy(filePath, narrow.c_str(), MAX_PATH - 1);

        std::ofstream f(p, std::ios::binary | std::ios::trunc);
        if (!f) {
            std::cerr << "Failed to create test attachment at " << narrow << std::endl;
            FreeLibrary(hDll);
            return 1;
        }
        f << "go-mapi test harness attachment payload\n";
    }
    char fileName[] = "test.txt";

    MapiFileDesc attachment = {};
    attachment.nPosition = 0;
    attachment.lpszPathName = filePath;
    attachment.lpszFileName = fileName;

    MapiRecipDesc recipient = {};
    recipient.ulRecipClass = MAPI_TO;
    recipient.lpszName = toName;
    recipient.lpszAddress = toAddress;

    MapiMessage message = {};
    message.lpszSubject = subject;
    message.lpszNoteText = body;
    message.nRecipCount = 1;
    message.lpRecips = &recipient;
    message.nFileCount = 1;
    message.lpFiles = &attachment;

    // Send the message
    ULONG result = MAPISendMail(0, 0, &message, 0, 0);

    std::cout << "MAPISendMail returned: " << result << std::endl;

    // Verify JSON file was created
    std::string tempDir = TestUtilities::GetGoMapiTempDir();
    bool success = TestUtilities::VerifyJsonFileCreated(tempDir);

    if (success) {
        // Find and validate the JSON file
        for (const auto& entry : std::filesystem::directory_iterator(tempDir)) {
            if (entry.path().extension() == ".json") {
                // Check that it contains attachment info
                success = TestUtilities::ValidateJsonFile(entry.path().string());
                break;
            }
        }
    }

    FreeLibrary(hDll);
    return success ? 0 : 1;
}
