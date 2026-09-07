# UI redesign visual QA

## Scope

The home discovery page and the document reader were rebuilt around a shared visual system inspired by Lightspark: warm paper background, restrained ink typography, teal accent, hairline separators, soft editorial imagery, and a single navigation/search model shared by public pages and the admin entry point.

## Evidence

- `artifacts/ui-qa/home-comparison.png` places the selected home concept beside the local implementation at the same review size.
- `artifacts/ui-qa/reader-comparison.png` places the selected reader concept beside the local implementation at the same review size.
- `artifacts/ui-qa/home-desktop.png` and `artifacts/ui-qa/reader-desktop.png` are the browser captures used for the comparison.

## Interaction checks

- Search dialog opens from the home hero and the reader header.
- Search is debounced, reports result counts, and navigates to a selected document.
- Reader navigation updates the URL without a full reload; previous/next links and the document tree stay in sync.
- Code copy reports success through the button label and an accessible status message.
- The admin route still opens the administrator setup screen and uses the shared header language.
- Responsive CSS covers the compact header, navigation drawer, stacked article layout, and collapsible table of contents.

## Verification

- `vue-tsc --noEmit -p web/tsconfig.json` passed.
- `vite build web` passed.
- The local preview served through Vite + the Go preview binary loaded the real tree, site metadata, search results, and article content.
- Go tests were not rerun in this environment because the Go executable is unavailable on the current PATH; the Docker build remains configured to run formatting, vet, and production tests in CI.

final result: passed
