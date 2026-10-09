#include <CoreGraphics/CoreGraphics.h>

#include "scale.h"

// FLTK draws on macOS into the Quartz context it keeps in fl_gc, scaled by
// its own screen scaling and the screen's backing scale (2 on a Retina
// screen). The context's transform says by how much: device pixels to the
// unit. FLTK flips the y axis, so the x scale is the one to read.
extern CGContextRef fl_gc;

double tlua_fltk_device_scale(void) {
  if (fl_gc == NULL) return 0;
  CGAffineTransform t = CGContextGetUserSpaceToDeviceSpaceTransform(fl_gc);
  double s = t.a < 0 ? -t.a : t.a;
  return s;
}
