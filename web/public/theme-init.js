// Applied before first paint so dark mode does not flash. Kept as a file (not inline) so the
// Content-Security-Policy can forbid inline scripts. Drops the retired design key.
try { var t = localStorage.getItem("stunsails-theme"); if (t === "dark" || (t !== "light" && matchMedia("(prefers-color-scheme: dark)").matches)) document.documentElement.classList.add("dark"); localStorage.removeItem("stunsails-design") } catch (e) {}
