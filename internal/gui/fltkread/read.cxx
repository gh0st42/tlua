#include "read.h"

// fl_read_image is a function in the library go-fltk links, which go-fltk
// itself does not wrap. Declaring it is enough for the linker to find it.
typedef unsigned char uchar;
uchar *fl_read_image(uchar *p, int X, int Y, int W, int H, int alpha = 0);

// tlua_fltk_read_image reads w by h pixels of RGB from the top left of the
// window or offscreen buffer being drawn into.
void tlua_fltk_read_image(unsigned char *buf, int w, int h) {
  fl_read_image(buf, 0, 0, w, h, 0);
}
