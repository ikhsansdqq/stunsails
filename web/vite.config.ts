import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": new URL("./src", import.meta.url).pathname } },
  // `npm run dev` talks to a local `stunsails serve` on 8098 (see README).
  server: { proxy: { "/api": "http://127.0.0.1:8098" } },
  build: {
    // The Go binary embeds dist/; keep .keep so the embed pattern never comes up empty.
    emptyOutDir: false,
    rollupOptions: {
      output: {
        manualChunks: (id) => (/node_modules\/(react|react-dom|scheduler|@tanstack\/react-query|@tanstack\/query-core|wouter|regexparam)\//.test(id) ? "vendor" : undefined),
      },
    },
  },
});
