#ifdef __cplusplus
extern "C" {
#endif

int tlua_fltk_send(int event, int x, int y, int x_root, int y_root, int dx, int dy,
                   int keysym, int state, int clicks, int is_click,
                   const char *text, int length);

#ifdef __cplusplus
}
#endif
