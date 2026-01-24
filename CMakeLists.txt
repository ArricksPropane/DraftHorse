cmake_minimum_required(VERSION 3.16)
project(go-mapi)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Build options
option(BUILD_TESTS "Build test harness" ON)

# Add the interceptor DLL
add_subdirectory(src/interceptor)

# Add the test harness
if(BUILD_TESTS)
    add_subdirectory(src/test-harness)
endif()

# Summary
message(STATUS "")
message(STATUS "go-mapi Build Configuration")
message(STATUS "============================")
message(STATUS "Build Tests: ${BUILD_TESTS}")
message(STATUS "")
