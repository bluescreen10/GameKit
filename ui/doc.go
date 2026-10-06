// Package ui defines a backend-neutral native user-interface protocol.
//
// Backend creates windows. Each Window owns its controls, dialogs, event
// dispatch, and presentation surfaces. Window and Canvas both implement
// Drawable, so a GPU backend can create a presentation surface for an entire
// window or for a region beside native controls.
package ui
