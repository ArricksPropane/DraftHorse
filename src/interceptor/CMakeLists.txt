cmake_minimum_required(VERSION 3.16)
project(go-mapi-interceptor)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Create the DLL
add_library(go-mapi SHARED
    src/main.cpp
    src/mapi_impl.cpp
    src/json_writer.cpp
    src/fs_utils.cpp
)

# Set DLL output directory
set_target_properties(go-mapi PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
)

# Link against Windows libraries
target_link_libraries(go-mapi PRIVATE
    kernel32.lib
    user32.lib
)

# Include directories
target_include_directories(go-mapi PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/../interceptor"
)

# Platform-specific settings
if(MSVC)
    # MSVC specific options
    target_compile_options(go-mapi PRIVATE /W4 /WX-)
    # Define module definition file
    set_target_properties(go-mapi PROPERTIES
        LINK_FLAGS "/DEF:\"${CMAKE_CURRENT_SOURCE_DIR}/mapi_exports.def\""
    )
endif()
