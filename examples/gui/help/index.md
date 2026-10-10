# Welcome

This window is a **MarkdownView** showing `help/index.md`. Everything in it
is Markdown: *italic*, **bold**, `code`, [links](editing.md), tables and
pictures.

[[toc]]

## Getting around

- [Editing text](editing.md) is another page; *Back* comes back here.
- [[Keyboard Shortcuts]] is a wiki link: the page `Keyboard Shortcuts.md`.
- [Shortcuts](editing.md#shortcuts) goes straight to a heading on it.
- [Pictures](#pictures), further down this page.
- [tlua on GitHub](https://github.com/gh0st42/tlua) opens in the browser.
- [Save the file](app:save) is a link the program answers itself.

Select text with the mouse and copy it with Ctrl+C (Cmd+C on a Mac). The
keys scroll: Up, Down, Page Up, Page Down, Space, Home and End.

> A quote, for a tip or a warning. Links in it work too: [editing](editing.md).

## Pictures

![The tlua logo](logo.png)

A picture is read from beside the page, out of the bundle when the program
is one.

## Tables

| Control | What it shows | Events |
|:--------|:-------------:|-------:|
| `Label` | a caption | none |
| `MarkdownView` | formatted text, with [[editing|links]] | `onLink`, `onNavigate`, `onHover` |
| `Canvas` | whatever `onDraw` draws, which can be a lot of things when a sentence goes on long enough to wrap | `onDraw` |

## Lists

1. First
2. Second
    - nested
    - and more
3. Third

---

The end of the page.
