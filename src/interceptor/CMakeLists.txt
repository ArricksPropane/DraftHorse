cmake_minimum_required(VERSION 3.16)
project(go-mapi-interceptor)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Build options
option(BUILD_TESTS "Build test harness" ON)

# Create the DLL
add_library(go-mapi SHARED
    main.cpp
    mapi_impl.cpp
    json_writer.cpp
    fs_utils.cpp
)

# Set DLL output directory
set_target_properties(go-mapi PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin"
    PREFIX ""  # Avoid "lib" prefix on MinGW
)

# Include directories
target_include_directories(go-mapi PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}"
)

# Platform-specific settings
if(MSVC)
    # MSVC specific options
    target_compile_options(go-mapi PRIVATE /W4 /WX-)
    target_link_libraries(go-mapi PRIVATE kernel32.lib user32.lib)
    set_target_properties(go-mapi PROPERTIES
        LINK_FLAGS "/DEF:\"${CMAKE_CURRENT_SOURCE_DIR}/mapi_exports.def\""
    )
elseif(MINGW)
    # MinGW specific options
    target_compile_options(go-mapi PRIVATE -Wall -Wextra -Wno-unused-parameter)
    target_link_libraries(go-mapi PRIVATE kernel32 user32 -static-libgcc -static-libstdc++)
    set_target_properties(go-mapi PROPERTIES
        LINK_FLAGS "-Wl,--output-def,${CMAKE_BINARY_DIR}/bin/go-mapi.def -static"
    )
    # Export symbols using the .def file
    target_sources(go-mapi PRIVATE mapi_exports.def)
endif()

# Add the test harness
if(BUILD_TESTS)
    add_subdirectory(test-harness)
endif()

# Summary
message(STATUS "")
message(STATUS "go-mapi Interceptor Build Configuration")
message(STATUS "========================================")
message(STATUS "Compiler: ${CMAKE_CXX_COMPILER_ID}")
message(STATUS "Build Tests: ${BUILD_TESTS}")
message(STATUS "")
