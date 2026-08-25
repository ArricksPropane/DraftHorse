#include "test_utils.h"
#include <shlobj.h>
#include <iostream>
#include <fstream>
#include <sstream>
#include <filesystem>
#include <regex>

namespace fs = std::filesystem;

namespace mapi_test {

MAPISendMailFunc TestUtilities::LoadMAPISendMail(const std::string& dllPath) {
    HMODULE hDll = LoadLibraryA(dllPath.c_str());
    if (!hDll) {
        std::cerr << "Failed to load DLL: " << dllPath << std::endl;
        return nullptr;
    }

    MAPISendMailFunc func = reinterpret_cast<MAPISendMailFunc>(
        GetProcAddress(hDll, "MAPISendMail")
    );

    if (!func) {
        std::cerr << "Failed to get MAPISendMail function pointer" << std::endl;
        FreeLibrary(hDll);
        return nullptr;
    }

    return func;
}

bool TestUtilities::VerifyJsonFileCreated(const std::string& tempDir) {
    try {
        for (const auto& entry : fs::directory_iterator(tempDir)) {
            if (entry.is_regular_file() && entry.path().extension() == ".json") {
                std::cout << "Found JSON file: " << entry.path().filename().string() << std::endl;
                return true;
            }
        }
    } catch (const std::exception& e) {
        std::cerr << "Error checking directory: " << e.what() << std::endl;
    }

    return false;
}

bool TestUtilities::ValidateJsonFile(const std::string& filePath) {
    try {
        std::ifstream file(filePath);
        if (!file.is_open()) {
            std::cerr << "Failed to open file: " << filePath << std::endl;
            return false;
        }

        std::stringstream buffer;
        buffer << file.rdbuf();
        std::string content = buffer.str();

        // Check for required fields
        std::vector<std::string> requiredFields = {
            "\"version\"",
            "\"timestamp\"",
            "\"subject\"",
            "\"body\"",
            "\"bodyFormat\"",
            "\"recipients\"",
            "\"attachments\"",
            "\"originApp\""
        };

        for (const auto& field : requiredFields) {
            if (content.find(field) == std::string::npos) {
                std::cerr << "Missing required field: " << field << std::endl;
                return false;
            }
        }

        // Check for valid JSON structure
        if (content.front() != '{' || content.back() != '}') {
            std::cerr << "Invalid JSON structure" << std::endl;
            return false;
        }

        std::cout << "JSON file valid: " << filePath << std::endl;
        return true;
    } catch (const std::exception& e) {
        std::cerr << "Error validating JSON: " << e.what() << std::endl;
        return false;
    }
}

void TestUtilities::CleanupTestFiles(const std::string& tempDir) {
    try {
        for (const auto& entry : fs::directory_iterator(tempDir)) {
            if (entry.is_regular_file() && entry.path().extension() == ".json") {
                fs::remove(entry);
                std::cout << "Deleted: " << entry.path().filename().string() << std::endl;
            }
        }
    } catch (const std::exception& e) {
        std::cerr << "Error cleaning up files: " << e.what() << std::endl;
    }
}

std::string TestUtilities::GetGoMapiTempDir() {
    // ARRICKS-05: this returned %TEMP%\DraftHorse, but the DLL moved its queue to
    // %LOCALAPPDATA%\DraftHorse\queue in quick/260423-msq. Every harness test has
    // since been watching a directory the DLL never writes to. The harness is
    // also not wired into CTest, so the breakage stayed invisible in CI.
    //
    // Must stay in step with FsUtils::GetBaseQueueDir().
    wchar_t basePath[MAX_PATH];
    HRESULT hr = SHGetFolderPathW(nullptr, CSIDL_LOCAL_APPDATA, nullptr,
                                  SHGFP_TYPE_CURRENT, basePath);
    if (FAILED(hr)) {
        return "";
    }

    std::string result;
    // Convert wide string to narrow string
    int size_needed = WideCharToMultiByte(CP_UTF8, 0, basePath, -1, NULL, 0, NULL, NULL);
    if (size_needed <= 1) {
        return "";
    }
    result.resize(size_needed - 1);
    WideCharToMultiByte(CP_UTF8, 0, basePath, -1, &result[0], size_needed, NULL, NULL);

    // Append the queue directory: %LOCALAPPDATA%\DraftHorse\queue
    result = fs::path(result).append("DraftHorse").append("queue").string();
    return result;
}

void TestUtilities::PrintTestResult(const std::string& testName, bool passed) {
    if (passed) {
        std::cout << "\n✓ [PASS] " << testName << std::endl;
    } else {
        std::cerr << "\n✗ [FAIL] " << testName << std::endl;
    }
}

int TestUtilities::GetJsonFileCount(const std::string& tempDir) {
    int count = 0;
    try {
        for (const auto& entry : fs::directory_iterator(tempDir)) {
            if (entry.is_regular_file() && entry.path().extension() == ".json") {
                count++;
            }
        }
    } catch (const std::exception& e) {
        std::cerr << "Error counting files: " << e.what() << std::endl;
    }
    return count;
}

std::string TestUtilities::ReadNewestJsonContent(const std::string& tempDir) {
    std::string newestPath;
    std::filesystem::file_time_type newestTime{};

    try {
        for (const auto& entry : fs::directory_iterator(tempDir)) {
            if (entry.is_regular_file() && entry.path().extension() == ".json") {
                auto wt = entry.last_write_time();
                if (newestPath.empty() || wt > newestTime) {
                    newestTime = wt;
                    newestPath = entry.path().string();
                }
            }
        }
    } catch (...) {}

    if (newestPath.empty()) return "";

    std::ifstream file(newestPath);
    std::stringstream buf;
    buf << file.rdbuf();
    return buf.str();
}

}  // namespace mapi_test
