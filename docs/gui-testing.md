# Desktop / admin UI (replaces Fyne `claudia-gui`)

The old **Fyne** nested module was removed in favor of:

1. **Gateway-served operator UI** — open **`http://<gateway>/ui/login`** (session cookie after you post a gateway token to **`POST /api/ui/login`**). Control panel: **`/ui/panel`**.
2. **Optional native shell** — build **`claudia-desktop`** with **`make desktop-build`** (**`-tags desktop`**, **CGO**, WebKitGTK / WebView2), then run **`./claudia-desktop desktop`** with the same flags as **`claudia serve`** (see **`make desktop-run`**).

Normative plan: [ui-tool.plan.md](ui-tool.plan.md).
