/* clang-format off */
/* hard.recipe.v1
source: "github.com/leethomason/tinyxml2"
build_system: "cmake"
source_directory: "."
configure_arguments:
  - "-DCMAKE_BUILD_TYPE=Release"
  - "-Dtinyxml2_SHARED_LIBS=OFF"
  - "-Dtinyxml2_BUILD_TESTING=OFF"
  - "-Dtinyxml2_INSTALL_PKGCONFIG=OFF"
  - "-DCMAKE_INSTALL_LIBDIR=lib"
  - "-DCMAKE_INSTALL_INCLUDEDIR=include"
source_include_directories:
  - "."
include_directories:
  - "include"
static_libraries:
  - "lib/libtinyxml2.a"
*/
/* clang-format on */
#pragma once

#include <tinyxml2.h>
