package ui

// FileFilter is one selectable group of filename patterns. Patterns use shell
// syntax such as "*.png" and "*.jpg".
type FileFilter struct {
	Name     string
	Patterns []string
}

// OpenFileOptions controls an open-file dialog. Directory and Filename are
// optional initial values.
type OpenFileOptions struct {
	Title     string
	Directory string
	Filename  string
	Filters   []FileFilter
}

// SaveFileOptions controls a save-file dialog. Directory and Filename are
// optional initial values.
type SaveFileOptions struct {
	Title            string
	Directory        string
	Filename         string
	Filters          []FileFilter
	ConfirmOverwrite bool
}

// DirectoryDialogOptions controls a directory-selection dialog.
type DirectoryDialogOptions struct {
	Title     string
	Directory string
}
