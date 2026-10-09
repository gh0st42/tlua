#include <string.h>

#include "input.h"

// FLTK keeps the event being handled in public static members of class Fl,
// and Fl::handle() dispatches an event the way one from the system would be.
// go-fltk has neither, but links the library that does: declaring the few
// members used here is enough for the linker to find them. Nothing else of
// FLTK's class is declared, so nothing else may be used.
class Fl_Window;
class Fl {
public:
  static int e_number, e_x, e_y, e_x_root, e_y_root, e_dx, e_dy;
  static int e_state, e_clicks, e_is_click, e_keysym, e_original_keysym, e_length;
  static char *e_text;
  static int handle(int event, Fl_Window *window);
  static Fl_Window *first_window();
  static Fl_Window *grab_;
};

// FLTK may read the text after the call that set it has returned.
static char text_buf[4096];

extern "C" int tlua_fltk_send(int event, int x, int y, int x_root, int y_root, int dx, int dy,
                              int keysym, int state, int clicks, int is_click,
                              const char *text, int length) {
  // A window holding the grab, as a popup menu does while it is up, is
  // where FLTK sends what happens, as it would the system's events.
  Fl_Window *w = Fl::grab_ ? Fl::grab_ : Fl::first_window();
  if (!w) {
    return -1;
  }
  if (length >= (int)sizeof(text_buf)) {
    length = sizeof(text_buf) - 1;
  }
  memcpy(text_buf, text, length);
  text_buf[length] = 0;
  Fl::e_x = x;
  Fl::e_y = y;
  Fl::e_x_root = x_root;
  Fl::e_y_root = y_root;
  Fl::e_dx = dx;
  Fl::e_dy = dy;
  Fl::e_keysym = keysym;
  Fl::e_original_keysym = keysym;
  Fl::e_state = state;
  Fl::e_clicks = clicks;
  Fl::e_is_click = is_click;
  Fl::e_text = text_buf;
  Fl::e_length = length;
  return Fl::handle(event, w);
}

extern "C" int tlua_fltk_grabbing(void) { return Fl::grab_ != 0; }
