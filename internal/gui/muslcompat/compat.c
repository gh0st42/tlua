// go-fltk's FLTK libraries were compiled against glibc, and call functions
// only glibc has by those names: the 64-bit file functions beside the plain
// ones, the checked printf and memcpy that _FORTIFY_SOURCE turns calls
// into, and the C23 scanf and strtol of glibc 2.38. On 64-bit Linux musl's
// plain file functions are already 64-bit, with the same structures, and
// the others differ from the plain ones only in what they check; so each is
// the plain one by another name. The checked ones still check what is cheap
// to: a copy or a print longer than its buffer stops the program, as
// glibc's would.

#define _GNU_SOURCE
#include <dirent.h>
#include <fcntl.h>
#include <setjmp.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>

// Older musl names some of these as macros for the plain ones.
#undef fopen64
#undef open64
#undef fcntl64
#undef stat64
#undef mkstemp64
#undef mkostemp64
#undef scandir64
#undef readdir64
#undef mmap64
#undef ftruncate64
#undef posix_fallocate64

/* ---- the 64-bit file functions ---- */

FILE *fopen64(const char *path, const char *mode) { return fopen(path, mode); }

int open64(const char *path, int flags, ...) {
  mode_t mode = 0;
  if (flags & O_CREAT) {
    va_list ap;
    va_start(ap, flags);
    mode = va_arg(ap, int);
    va_end(ap);
  }
  return open(path, flags, mode);
}

int fcntl64(int fd, int cmd, ...) {
  va_list ap;
  va_start(ap, cmd);
  long arg = va_arg(ap, long);
  va_end(ap);
  return fcntl(fd, cmd, arg);
}

int stat64(const char *path, struct stat *buf) { return stat(path, buf); }

// __xstat64 is what an older glibc compiled stat64() down to, with a
// version of the structure first; there is only the one.
int __xstat64(int version, const char *path, struct stat *buf) {
  (void)version;
  return stat(path, buf);
}

int mkstemp64(char *template) { return mkstemp(template); }
int mkostemp64(char *template, int flags) { return mkostemp(template, flags); }

int scandir64(const char *dir, struct dirent ***list, int (*filter)(const struct dirent *),
              int (*compare)(const struct dirent **, const struct dirent **)) {
  return scandir(dir, list, filter, compare);
}

struct dirent *readdir64(DIR *dir) { return readdir(dir); }

void *mmap64(void *addr, size_t len, int prot, int flags, int fd, off_t off) {
  return mmap(addr, len, prot, flags, fd, off);
}

int ftruncate64(int fd, off_t len) { return ftruncate(fd, len); }
int posix_fallocate64(int fd, off_t off, off_t len) { return posix_fallocate(fd, off, len); }

/* ---- C23's scanf and strtol, which only differ in reading 0b ---- */

int __isoc23_sscanf(const char *s, const char *format, ...) {
  va_list ap;
  va_start(ap, format);
  int n = vsscanf(s, format, ap);
  va_end(ap);
  return n;
}

int __isoc23_vsscanf(const char *s, const char *format, va_list ap) { return vsscanf(s, format, ap); }

int __isoc23_fscanf(FILE *f, const char *format, ...) {
  va_list ap;
  va_start(ap, format);
  int n = vfscanf(f, format, ap);
  va_end(ap);
  return n;
}

long __isoc23_strtol(const char *s, char **end, int base) { return strtol(s, end, base); }
long long __isoc23_strtoll(const char *s, char **end, int base) { return strtoll(s, end, base); }
unsigned long __isoc23_strtoul(const char *s, char **end, int base) { return strtoul(s, end, base); }

/* ---- the checked functions of _FORTIFY_SOURCE ---- */

void *__memcpy_chk(void *dst, const void *src, size_t n, size_t room) {
  if (n > room) abort();
  return memcpy(dst, src, n);
}

int __fprintf_chk(FILE *f, int flag, const char *format, ...) {
  (void)flag;
  va_list ap;
  va_start(ap, format);
  int n = vfprintf(f, format, ap);
  va_end(ap);
  return n;
}

int __vsnprintf_chk(char *s, size_t max, int flag, size_t room, const char *format, va_list ap) {
  (void)flag;
  if (max > room) abort();
  return vsnprintf(s, max, format, ap);
}

int __snprintf_chk(char *s, size_t max, int flag, size_t room, const char *format, ...) {
  va_list ap;
  va_start(ap, format);
  int n = __vsnprintf_chk(s, max, flag, room, format, ap);
  va_end(ap);
  return n;
}

int __sprintf_chk(char *s, int flag, size_t room, const char *format, ...) {
  (void)flag;
  va_list ap;
  va_start(ap, format);
  int n = vsnprintf(s, room, format, ap);
  va_end(ap);
  if (n >= 0 && (size_t)n >= room) abort();
  return n;
}

int __vasprintf_chk(char **s, int flag, const char *format, va_list ap) {
  (void)flag;
  return vasprintf(s, format, ap);
}

int __asprintf_chk(char **s, int flag, const char *format, ...) {
  (void)flag;
  va_list ap;
  va_start(ap, format);
  int n = vasprintf(s, format, ap);
  va_end(ap);
  return n;
}

// glibc's and musl's jmp_buf are the same size on 64-bit Linux, and setjmp
// fills it in with musl's own; so musl's longjmp reads it back.
void __longjmp_chk(jmp_buf env, int val) { longjmp(env, val); }
