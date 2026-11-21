//go:build embednative && cgo

// link_cxx.cpp is empty on purpose: a C++ file makes cgo link with $CXX,
// which brings the C++ runtime llama.cpp needs (libstdc++ with g++, libc++
// with zig c++) without naming it in LDFLAGS.
