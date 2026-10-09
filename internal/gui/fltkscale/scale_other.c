//go:build !darwin

#include "scale.h"

// Elsewhere FLTK's screen scale is the whole of it, and go-fltk says that.
double tlua_fltk_device_scale(void) { return 0; }
