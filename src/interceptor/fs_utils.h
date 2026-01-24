#pragma once

#include <windows.h>
#include <string>

namespace go_mapi {

class FsUtils {
public:
    // Get the temp directory path (e.g., %TEMP%\go-mapi\)
    static std::wstring GetTempPath();

    // Ensure the output directory exists, create if necessary
    static bool EnsureOutputDirectory();

    // Generate a unique filename (timestamp_randomchars.json)
    static std::wstring GenerateUniqueFilename();

    // Write UTF-8 encoded content to a file
    static bool WriteFile(const std::wstring& filePath, const std::string& content);

private:
    // Get base temp directory
    static std::wstring GetBaseTempDir();

    // Get 6 random hex characters
    static std::string GetRandomSuffix();
};

} // namespace go_mapi
