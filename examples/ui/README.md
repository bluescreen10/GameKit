# GameKit widget tester

This example displays the native widgets implemented by the `ui` package. It
uses Cocoa on macOS, Win32 on Windows, and GTK 3 on Linux.

The tabs cover editable controls, selection controls, images, a native scroll
view, and a GPU-presentable canvas. The scrolling page can be moved with the
mouse wheel, native scrollbars, or the Top/Middle/Bottom buttons.

Run it on macOS or Windows:

```sh
go run ./examples/ui
```

Run it on Linux after installing the GTK 3 development package:

```sh
go run -tags gtk ./examples/ui
```

The Canvas tab is intentionally blank. It exposes a native surface that a GPU
backend can use for 2D or 3D presentation.
