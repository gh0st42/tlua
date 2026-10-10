#include "list.h"

// FLTK keeps the widget below the mouse, and the event's position, in
// public static members of class Fl; a browser finds its item at a height
// with find_item and numbers it with lineno. go-fltk wraps none of them,
// but links the library that has them: declaring these few is enough for
// the linker. Nothing else of the classes is declared, so nothing else may
// be used.
class Fl_Widget;
class Fl {
public:
  static Fl_Widget *belowmouse_;
  static int e_y;
};
class Fl_Browser_ {
public:
  void *find_item(int ypos);
};
class Fl_Browser : public Fl_Browser_ {
public:
  int lineno(void *item) const;
};

int tlua_fltk_line_under_mouse(void) {
  Fl_Browser *b = (Fl_Browser *)Fl::belowmouse_;
  if (!b) return 0;
  void *item = b->find_item(Fl::e_y);
  return item ? b->lineno(item) : 0;
}
