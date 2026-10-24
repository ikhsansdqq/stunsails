import { defineConfig } from "vitepress";

// The public repository. Change this one line when the repo moves.
const repo = "https://github.com/stunsails/stunsails";

export default defineConfig({
  title: "StunSails",
  description: "One small service that watches a server and every Docker container on it, and lets its owner manage them.",
  // GitHub Pages serves a project site under /<repo>/; DOCS_BASE overrides it.
  base: process.env.DOCS_BASE ?? "/",
  cleanUrls: true,
  // Needs git history, which the alpine build image has none of.
  lastUpdated: !!process.env.CI,
  head: [["link", { rel: "icon", href: "/favicon.svg" }]],
  themeConfig: {
    logo: "/favicon.svg",
    search: { provider: "local" },
    nav: [
      { text: "Guide", link: "/guide/introduction" },
      { text: "Configuration", link: "/configuration/environment" },
      { text: "Features", link: "/features/containers" },
      { text: "Security", link: "/security" },
    ],
    sidebar: [
      {
        text: "Getting started",
        items: [
          { text: "Introduction", link: "/guide/introduction" },
          { text: "Requirements", link: "/guide/requirements" },
          { text: "Install", link: "/guide/install" },
          { text: "First sign-in", link: "/guide/first-sign-in" },
          { text: "Reverse proxy", link: "/guide/reverse-proxy" },
          { text: "Upgrade and uninstall", link: "/guide/upgrade" },
        ],
      },
      {
        text: "Configuration",
        items: [
          { text: "Environment variables", link: "/configuration/environment" },
          { text: "systemd unit", link: "/configuration/systemd" },
          { text: "Alerts", link: "/configuration/alerts" },
        ],
      },
      {
        text: "Features",
        items: [
          { text: "Containers", link: "/features/containers" },
          { text: "Compose stacks", link: "/features/stacks" },
          { text: "Storage and cleanup", link: "/features/storage" },
          { text: "Sites and certificates", link: "/features/sites" },
          { text: "Activity", link: "/features/activity" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "Security model", link: "/security" },
          { text: "Troubleshooting", link: "/troubleshooting" },
          { text: "FAQ", link: "/faq" },
          { text: "Contributing", link: "/contributing" },
        ],
      },
    ],
    socialLinks: [{ icon: "github", link: repo }],
    editLink: { pattern: `${repo}/edit/main/docs/:path` },
    footer: { message: "Released under the GNU AGPL v3 or later.", copyright: "Copyright © 2026 Ikhsan Assidiqie" },
  },
});
