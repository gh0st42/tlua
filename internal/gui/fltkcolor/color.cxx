#include "color.h"

// FLTK's colour chooser is a function in the library go-fltk links, which
// go-fltk itself does not wrap. Declaring it is enough for the linker to
// find it.
typedef unsigned char uchar;
int fl_color_chooser(const char *name, uchar &r, uchar &g, uchar &b, int m = -1);

int tlua_fltk_choose_color(const char *title, unsigned char *r, unsigned char *g, unsigned char *b) {
  return fl_color_chooser(title, *r, *g, *b);
}
