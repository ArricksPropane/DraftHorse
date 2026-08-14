// ARRICKS-04: coverage for MAPISendDocuments.
//
// This entry point was a stub that returned SUCCESS_SUCCESS without queueing
// anything, so any caller using it lost mail silently. It is a common choice
// for scanner / MFP "Scan to Email" software, which makes it the single most
// important path to verify on this deployment.
//
// The test drives the real exported symbol with two genuine files and a
// semicolon-delimited list, then confirms a queue JSON was produced and that
// both attachment copies landed in the sibling directory.

#include <windows.h>
#include <cstring>
#include <fstream>
#include <iostream>
#include <string>
#include <vector>
#include <filesystem>
#include "../test_utils.h"

using namespace mapi_test;

namespace {

// Signature of the real export (ANSI-only; there is no wide variant).
typedef ULONG(WINAPI* MAPISendDocumentsFunc)(ULONG_PTR ulUIParam,
                                             LPSTR lpszDelimChar,
                                             LPSTR lpszFilePaths,
                                             LPSTR lpszFileNames,
                                             ULONG ulReserved);

bool WriteScratchFile(const std::filesystem::path& p, const std::string& body) {
    std::ofstream f(p, std::ios::binary | std::ios::trunc);
    if (!f) return false;
    f << body;
    return true;
}

}  // namespace

int test_send_documents() {
    std::cout << "\nTest: MAPISendDocuments (scanner path)" << std::endl;

    HMODULE hDll = LoadLibraryA("go-mapi.dll");
    if (!hDll) {
        std::cerr << "Failed to load go-mapi.dll" << std::endl;
        return 1;
    }

    MAPISendDocumentsFunc MAPISendDocuments =
        reinterpret_cast<MAPISendDocumentsFunc>(
            GetProcAddress(hDll, "MAPISendDocuments"));

    if (!MAPISendDocuments) {
        std::cerr << "Failed to get MAPISendDocuments function" << std::endl;
        FreeLibrary(hDll);
        return 1;
    }

    // Two real files, mimicking a two-page scan delivered as separate PDFs.
    wchar_t tempPathW[MAX_PATH];
    GetTempPathW(MAX_PATH, tempPathW);
    std::filesystem::path tempDirPath(tempPathW);
    std::filesystem::path fileA = tempDirPath / "go-mapi-harness-scan-1.pdf";
    std::filesystem::path fileB = tempDirPath / "go-mapi-harness-scan-2.pdf";

    if (!WriteScratchFile(fileA, "%PDF-1.4 harness page one\n") ||
        !WriteScratchFile(fileB, "%PDF-1.4 harness page two\n")) {
        std::cerr << "Failed to create scratch scan files" << std::endl;
        FreeLibrary(hDll);
        return 1;
    }

    std::string pathsStr = fileA.string() + ";" + fileB.string();
    std::string namesStr = "Scan 1.pdf;Scan 2.pdf";

    std::vector<char> paths(pathsStr.begin(), pathsStr.end());
    paths.push_back('\0');
    std::vector<char> names(namesStr.begin(), namesStr.end());
    names.push_back('\0');
    char delim[] = ";";

    int jsonCountBefore =
        TestUtilities::GetJsonFileCount(TestUtilities::GetGoMapiTempDir());

    ULONG result =
        MAPISendDocuments(0, delim, paths.data(), names.data(), 0);

    std::cout << "MAPISendDocuments returned: " << result << std::endl;

    // The stub returned SUCCESS_SUCCESS too, so the return code alone proves
    // nothing — the queue file is the actual assertion.
    bool success = (result == SUCCESS_SUCCESS);

    std::string queueDir = TestUtilities::GetGoMapiTempDir();
    int jsonCountAfter = TestUtilities::GetJsonFileCount(queueDir);
    if (jsonCountAfter <= jsonCountBefore) {
        std::cerr << "No queue JSON was produced — MAPISendDocuments did not "
                     "queue the message (regression to the stub behaviour?)"
                  << std::endl;
        success = false;
    }

    if (success) {
        std::string content = TestUtilities::ReadNewestJsonContent(queueDir);
        if (content.find("Scan 1.pdf") == std::string::npos ||
            content.find("Scan 2.pdf") == std::string::npos) {
            std::cerr << "Queue JSON is missing one or both display names"
                      << std::endl;
            success = false;
        }
    }

    // Confirm the copies actually landed: exactly one <stem> directory should
    // hold both files, since the DLL copies before writing the JSON.
    if (success) {
        bool foundBoth = false;
        for (const auto& entry : std::filesystem::directory_iterator(queueDir)) {
            if (!entry.is_directory()) continue;
            if (entry.path().filename() == "errors") continue;
            bool a = std::filesystem::exists(entry.path() / "Scan 1.pdf");
            bool b = std::filesystem::exists(entry.path() / "Scan 2.pdf");
            if (a && b) {
                foundBoth = true;
                break;
            }
        }
        if (!foundBoth) {
            std::cerr << "Attachment copies were not found in the queue's "
                         "per-message directory"
                      << std::endl;
            success = false;
        }
    }

    std::error_code ec;
    std::filesystem::remove(fileA, ec);
    std::filesystem::remove(fileB, ec);

    FreeLibrary(hDll);
    return success ? 0 : 1;
}
