import { useEffect, useState, type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Activity, Boxes, Globe, HardDrive, LayoutDashboard, LogOut, Menu, Moon, Package, Settings, Sun, X, type LucideIcon } from "@/components/ui/lucide";
import { SearchButton } from "@/components/CommandPalette";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useTheme } from "@/hooks/use-theme";

/** One sidebar row: icon, label and an optional count, as in freelance-dashboard's sidebar. */
const sidebarRow =
  "flex min-h-[34px] items-center gap-2.5 rounded-md px-2.5 text-sm whitespace-nowrap text-sidebar-ink outline-none transition-colors hover:bg-sidebar-hover hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-[current=page]:bg-sidebar-active aria-[current=page]:font-medium aria-[current=page]:text-foreground";

interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
  match: (p: string) => boolean;
}

const nav: NavItem[] = [
  { href: "/", label: "Overview", icon: LayoutDashboard, match: (p) => p === "/" },
  { href: "/containers", label: "Containers", icon: Package, match: (p) => p.startsWith("/containers") },
  { href: "/stacks", label: "Stacks", icon: Boxes, match: (p) => p.startsWith("/stacks") },
  { href: "/sites", label: "Sites", icon: Globe, match: (p) => p.startsWith("/sites") },
  { href: "/storage", label: "Storage", icon: HardDrive, match: (p) => p.startsWith("/storage") },
  { href: "/activity", label: "Activity", icon: Activity, match: (p) => p.startsWith("/activity") },
];
const settingsItem: NavItem = { href: "/settings", label: "Settings", icon: Settings, match: (p) => p.startsWith("/settings") };

/** StunSails' mark: the pixel band's orange as rising bars on black. */
export function StunSailsMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("stunsails-mark shrink-0", className)} aria-hidden>
      <rect width="32" height="32" rx="7" className="fill-foreground" />
      <g fill="#ff8205">
        <rect x="6" y="20" width="4" height="6" rx="1" />
        <rect x="11" y="16" width="4" height="10" rx="1" />
        <rect x="16" y="12" width="4" height="14" rx="1" />
        <rect x="21" y="7" width="4" height="19" rx="1" />
      </g>
    </svg>
  );
}

function crumbs(path: string): { title: string; parent?: NavItem } {
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "containers" && parts[1]) return { title: decodeURIComponent(parts[1]).slice(0, 40), parent: nav[1] };
  if (parts[0] === "stacks" && parts[1]) return { title: decodeURIComponent(parts[1]), parent: nav[2] };
  if (parts[0] === "sites" && parts[1]) return { title: decodeURIComponent(parts[1]), parent: nav[3] };
  const item = [...nav, settingsItem].find((i) => i.match(path));
  return { title: item?.label ?? "" };
}

function Sidebar({ mobile, onNavigate }: { mobile: boolean; onNavigate?: () => void }) {
  const [location] = useLocation();
  const { resolved, setTheme } = useTheme();
  const queryClient = useQueryClient();
  // The unhealthy count rides on the overview, which the shell already polls.
  const overview = useQuery({ queryKey: ["overview"], queryFn: api.overview, refetchInterval: 10_000 });
  const unhealthy = (overview.data?.counts.unhealthy ?? 0) + (overview.data?.counts.looping ?? 0);
  const sitesDown = overview.data?.sites?.down.length ?? 0;

  async function signOut() {
    await api.logout().catch(() => {});
    queryClient.setQueryData(["session"], { authenticated: false, configured: true });
    queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== "session" });
    toast.success("Signed out");
  }

  function toggleTheme() {
    const next = resolved === "dark" ? "light" : "dark";
    setTheme(next);
    toast(next === "dark" ? "Dark mode on" : "Light mode on");
  }

  const link = (item: NavItem) => {
    const Icon = item.icon;
    return (
      <Link key={item.href} href={item.href} onClick={onNavigate} aria-current={item.match(location) ? "page" : undefined} className={cn(sidebarRow, "group")}>
        <Icon size={17} className="shrink-0 text-muted-foreground group-aria-[current=page]:text-foreground" aria-hidden />
        <span className="flex-1">{item.label}</span>
        {item.href === "/containers" && unhealthy > 0 && (
          <span className="num rounded-full bg-critical px-1.5 text-[11px] leading-[18px] font-semibold text-on-tone" title={`${unhealthy} unhealthy or restarting`}>
            {unhealthy}
          </span>
        )}
        {item.href === "/sites" && sitesDown > 0 && (
          <span className="num rounded-full bg-critical px-1.5 text-[11px] leading-[18px] font-semibold text-on-tone" title={`${sitesDown} down`}>
            {sitesDown}
          </span>
        )}
      </Link>
    );
  };

  return (
    <aside aria-label="Primary navigation" className={cn("flex h-dvh w-60 shrink-0 flex-col overflow-hidden border-r bg-sidebar", mobile ? "" : "sticky top-0 hidden lg:flex")}>
      <div className="flex h-14 shrink-0 items-center gap-1 border-b px-2.5">
        <Link href="/" onClick={onNavigate} aria-label="StunSails, overview" className="flex min-w-0 flex-1 items-center gap-2 rounded-md p-1.5 outline-none hover:bg-sidebar-hover focus-visible:ring-[3px] focus-visible:ring-ring/50">
          <StunSailsMark className="size-6" />
          <span className="truncate text-sm font-semibold tracking-[-0.01em]">StunSails</span>
        </Link>
        {mobile && (
          <button type="button" onClick={onNavigate} aria-label="Close navigation" className="grid size-7 place-items-center rounded-md text-muted-foreground hover:bg-sidebar-hover hover:text-foreground">
            <X size={16} aria-hidden />
          </button>
        )}
      </div>
      <nav aria-label="Main" className="flex min-h-0 flex-1 flex-col gap-px overflow-y-auto px-2.5 py-2">
        {nav.map(link)}
      </nav>
      {overview.data?.info.hostname && (
        <p className="truncate px-5 pb-2 text-xs text-muted-foreground" title={overview.data.info.os}>
          {overview.data.info.hostname} · Docker {overview.data.docker.version || "?"}
        </p>
      )}
      {/* The AGPL asks that everyone using StunSails over the network can get its source. */}
      <a href="https://github.com/stunsails/stunsails" target="_blank" rel="noreferrer" className="truncate px-5 pb-2 text-xs text-muted-foreground hover:text-foreground">
        Source code · AGPL-3.0
      </a>
      <div className="flex flex-col gap-px border-t px-2.5 py-2">
        {link(settingsItem)}
        <button type="button" className={sidebarRow} onClick={toggleTheme}>
          {resolved === "dark" ? <Sun size={17} className="text-muted-foreground" aria-hidden /> : <Moon size={17} className="text-muted-foreground" aria-hidden />}
          {resolved === "dark" ? "Light mode" : "Dark mode"}
        </button>
        <button type="button" className={sidebarRow} onClick={() => void signOut()}>
          <LogOut size={17} className="text-muted-foreground" aria-hidden /> Sign out
        </button>
      </div>
    </aside>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  const [location] = useLocation();
  const [open, setOpen] = useState(false);
  const crumb = crumbs(location);
  // Container pages are linked by id; show the name once the page has loaded it (never fetched here).
  const ctrId = location.startsWith("/containers/") ? decodeURIComponent(location.split("/")[2] ?? "") : "";
  const ctr = useQuery({ queryKey: ["container", ctrId], queryFn: () => api.container(ctrId), enabled: false });
  if (ctrId) crumb.title = ctr.data?.detail.name ?? (/^[0-9a-f]{64}$/.test(ctrId) ? "Container" : ctrId);
  const [menuPath, setMenuPath] = useState(location);
  if (menuPath !== location) {
    setMenuPath(location);
    setOpen(false);
  }
  useEffect(() => {
    document.title = crumb.title ? `${crumb.title} · StunSails` : "StunSails";
  }, [crumb.title]);

  return (
    <div className="flex min-h-svh">
      <Sidebar mobile={false} />
      <div className={cn("fixed inset-0 z-40 transition-[visibility] lg:hidden", open ? "visible duration-200" : "invisible duration-150")}>
        <div className={cn("absolute inset-0 bg-black/40 transition-opacity ease-out", open ? "opacity-100 duration-200" : "opacity-0 duration-150")} onClick={() => setOpen(false)} aria-hidden />
        <div className={cn("relative h-full w-60 shadow-xl transition-[translate] ease-[cubic-bezier(0.32,0.72,0,1)]", open ? "translate-x-0 duration-200" : "-translate-x-full duration-150")}>
          <Sidebar mobile onNavigate={() => setOpen(false)} />
        </div>
      </div>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex h-14 items-center gap-3 border-b bg-header/85 px-4 backdrop-blur md:px-6">
          <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setOpen(true)} aria-label="Open navigation">
            <Menu size={16} aria-hidden />
          </Button>
          <nav aria-label="Breadcrumb" className="flex min-w-0 flex-1 items-center gap-2 text-sm">
            {crumb.parent && (
              <>
                <Link href={crumb.parent.href} className="flex shrink-0 items-center gap-2 rounded-md px-1.5 py-1 text-muted-foreground hover:bg-sidebar-hover hover:text-foreground">
                  <crumb.parent.icon size={16} className="shrink-0" aria-hidden />
                  {crumb.parent.label}
                </Link>
                <span className="text-muted-foreground/60" aria-hidden>
                  /
                </span>
              </>
            )}
            {crumb.title && (
              <strong aria-current="page" className="truncate font-medium">
                {crumb.title}
              </strong>
            )}
          </nav>
          <SearchButton />
          <LiveDot />
        </header>
        <ConnectionBanner />
        <main className="mx-auto w-full max-w-[1400px] flex-1 p-4 md:p-6">{children}</main>
      </div>
    </div>
  );
}

/** A small "Live" mark in the header: the pages refresh on their own. */
function LiveDot() {
  return (
    <span className="flex items-center gap-1.5 text-xs text-muted-foreground" title="Readings refresh every few seconds">
      <span className="relative flex size-2">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-good opacity-60 motion-reduce:animate-none" />
        <span className="relative inline-flex size-2 rounded-full bg-good" />
      </span>
      Live
    </span>
  );
}

function ConnectionBanner() {
  const [down, setDown] = useState(!navigator.onLine);
  useEffect(() => {
    const off = () => setDown(true);
    const on = () => setDown(false);
    addEventListener("offline", off);
    addEventListener("online", on);
    return () => {
      removeEventListener("offline", off);
      removeEventListener("online", on);
    };
  }, []);
  if (!down) return null;
  return (
    <div role="status" className="border-b border-warning/30 bg-warning/10 px-4 py-2 text-center text-sm text-warning md:px-6">
      You're offline. Readings resume when the connection is back.
    </div>
  );
}

/** Page title row, after freelance-dashboard's PageHeader. */
export function PageHeader({ title, sub, back, children }: { title: ReactNode; sub?: ReactNode; back?: ReactNode; children?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
      <div className="min-w-0">
        {back}
        <h1 className="text-2xl font-semibold tracking-[-0.025em] break-words md:text-[2rem] md:leading-tight">{title}</h1>
        {sub && <div className="mt-1.5 max-w-[70ch] text-[0.9375rem] text-muted-foreground">{sub}</div>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  );
}
