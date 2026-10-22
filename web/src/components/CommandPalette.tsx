import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { Command } from "cmdk";
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocation } from "wouter";
import { toast } from "sonner";
import {
  Activity,
  ArrowDownToLine,
  Bell,
  Boxes,
  Globe,
  HardDrive,
  LayoutDashboard,
  LogOut,
  Moon,
  Package,
  RefreshCw,
  RotateCw,
  ScrollText,
  Search,
  Settings,
  Sun,
  type LucideIcon,
} from "@/components/ui/lucide";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { formatBytes } from "@/lib/format";
import { useTheme } from "@/hooks/use-theme";
import { useContainerAction } from "@/components/ContainerActions";
import { useStackRun } from "@/components/StackRunner";
import { useRun } from "@/components/guard";

const Ctx = createContext<{ open: () => void }>({ open: () => {} });

const mac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);
const modKey = mac ? "⌘" : "Ctrl";

/** ⌘K / Ctrl+K (or /) from anywhere: jump to a page, container or stack, or run an action. */
export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && (e.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName));
      if ((e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey)) || (e.key === "/" && !typing)) {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);
  return (
    <Ctx.Provider value={{ open: () => setOpen(true) }}>
      {children}
      <DialogPrimitive.Root open={open} onOpenChange={setOpen}>
        <DialogPrimitive.Portal>
          <DialogPrimitive.Backdrop className="fixed inset-0 z-[90] bg-black/40 transition-opacity duration-150 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
          <DialogPrimitive.Popup className="fixed top-[14vh] left-1/2 z-[91] w-[calc(100vw-1.5rem)] max-w-2xl -translate-x-1/2 overflow-hidden rounded-xl border bg-card text-card-foreground shadow-2xl outline-none transition-[opacity,scale] duration-150 data-[ending-style]:scale-98 data-[ending-style]:opacity-0 data-[starting-style]:scale-98 data-[starting-style]:opacity-0 dark:ring-1 dark:ring-white/5">
            <DialogPrimitive.Title className="sr-only">Go to or run</DialogPrimitive.Title>
            {open && <Palette close={() => setOpen(false)} />}
          </DialogPrimitive.Popup>
        </DialogPrimitive.Portal>
      </DialogPrimitive.Root>
    </Ctx.Provider>
  );
}

const groupClass =
  "[&_[cmdk-group-heading]]:px-2.5 [&_[cmdk-group-heading]]:pt-3 [&_[cmdk-group-heading]]:pb-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-muted-foreground";
const itemClass =
  "flex cursor-default items-center gap-3 rounded-md px-2 py-1.5 text-sm outline-none select-none data-[selected=true]:bg-sidebar-active data-[selected=true]:text-foreground [&[data-selected=true]_[data-tile]]:border-foreground/15";

const pages: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/containers", label: "Containers", icon: Package },
  { href: "/stacks", label: "Stacks", icon: Boxes },
  { href: "/sites", label: "Sites", icon: Globe },
  { href: "/storage", label: "Storage", icon: HardDrive },
  { href: "/activity", label: "Activity", icon: Activity },
  { href: "/settings", label: "Settings", icon: Settings },
];

function Tile({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <span data-tile className="grid size-8 shrink-0 place-items-center rounded-md border bg-background text-muted-foreground">
      <Icon size={16} aria-hidden />
    </span>
  );
}

function Palette({ close }: { close: () => void }) {
  const [, navigate] = useLocation();
  const queryClient = useQueryClient();
  const containers = useQuery({ queryKey: ["containers"], queryFn: api.containers });
  const stacks = useQuery({ queryKey: ["stacks"], queryFn: api.stacks });
  const act = useContainerAction();
  const runStack = useStackRun();
  const run = useRun();
  const { resolved, setTheme } = useTheme();
  const go = (href: string) => {
    close();
    navigate(href);
  };
  const later = (fn: () => void) => {
    close();
    // Let the palette close before a confirm dialog opens.
    setTimeout(fn, 120);
  };

  const item = (value: string, icon: LucideIcon, label: string, hint: string, onSelect: () => void) => (
    <Command.Item key={value} value={value} onSelect={onSelect} className={itemClass}>
      <Tile icon={icon} />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span className="shrink-0 text-xs text-muted-foreground">{hint}</span>
    </Command.Item>
  );

  return (
    <Command loop label="Go to or run">
      <div className="flex items-center gap-3 border-b px-5">
        <Search size={20} className="shrink-0 text-muted-foreground" aria-hidden />
        <Command.Input autoFocus placeholder="Go to a container, stack or page, or run an action…" aria-label="Search Harbor" className="h-14 w-full bg-transparent text-base outline-none placeholder:text-muted-foreground" />
        <kbd className="hidden shrink-0 rounded border bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground sm:inline">Esc</kbd>
      </div>
      <Command.List className="max-h-[min(60vh,30rem)] scroll-py-2 overflow-y-auto overscroll-contain px-2.5 pb-2.5">
        <Command.Empty className="px-3 py-10 text-center text-sm text-muted-foreground">Nothing matches.</Command.Empty>
        <Command.Group heading="Pages" className={groupClass}>
          {pages.map((p) => item(`page ${p.label}`, p.icon, p.label, "Page", () => go(p.href)))}
        </Command.Group>
        {(containers.data?.length ?? 0) > 0 && (
          <Command.Group heading="Containers" className={groupClass}>
            {containers.data!.map((c) => item(`container ${c.name} ${c.project}`, Package, c.name, c.state, () => go(`/containers/${c.id}`)))}
            {containers.data!.map((c) => item(`logs ${c.name}`, ScrollText, `Logs: ${c.name}`, "Logs", () => go(`/containers/${c.id}?tab=logs`)))}
            {containers.data!
              .filter((c) => c.state === "running")
              .map((c) => item(`restart ${c.name}`, RotateCw, `Restart ${c.name}`, "Action", () => later(() => void act(c.id, c.name, "restart"))))}
          </Command.Group>
        )}
        {(stacks.data?.length ?? 0) > 0 && (
          <Command.Group heading="Stacks" className={groupClass}>
            {stacks.data!.map((s) => item(`stack ${s.project}`, Boxes, s.project, s.state, () => go(`/stacks/${s.project}`)))}
            {stacks.data!
              .filter((s) => s.manageable)
              .map((s) =>
                s.state === "running"
                  ? item(`restart stack ${s.project}`, RotateCw, `Restart ${s.project}`, "Action", () => later(() => void runStack(s.project, "restart")))
                  : item(`up stack ${s.project}`, ArrowDownToLine, `Bring ${s.project} up`, "Action", () => later(() => void runStack(s.project, "up"))),
              )}
          </Command.Group>
        )}
        <Command.Group heading="Actions" className={groupClass}>
          {item("measure storage disk", RefreshCw, "Measure Docker's disk use again", "Action", () =>
            later(() => {
              const id = toast.loading("Measuring Docker's disk use…");
              api
                .storage(true)
                .then((d) => {
                  queryClient.setQueryData(["storage"], d);
                  toast.success(`Docker uses ${formatBytes(d.df?.total_bytes ?? 0)}`, { id });
                })
                .catch((e: Error) => toast.error(e.message, { id }));
            }),
          )}
          {item("send test alert", Bell, "Send a test alert", "Action", () =>
            later(() => void run(() => api.testAlert(), { success: "Test alert sent" })),
          )}
          {item("theme dark light", resolved === "dark" ? Sun : Moon, resolved === "dark" ? "Light mode" : "Dark mode", "Action", () => {
            close();
            setTheme(resolved === "dark" ? "light" : "dark");
            toast(resolved === "dark" ? "Light mode on" : "Dark mode on");
          })}
          {item("sign out log out", LogOut, "Sign out", "Action", () => {
            close();
            void api.logout().finally(() => {
              queryClient.setQueryData(["session"], { authenticated: false, configured: true });
              toast.success("Signed out");
            });
          })}
        </Command.Group>
      </Command.List>
      <div className="flex items-center gap-4 border-t px-5 py-2.5 text-xs text-muted-foreground">
        <span>↑↓ to move</span>
        <span>↵ to open</span>
        <span className="ml-auto">{modKey} K anywhere</span>
      </div>
    </Command>
  );
}

/** The header's search field: opens the palette. */
export function SearchButton({ className }: { className?: string }) {
  const { open } = useContext(Ctx);
  return (
    <button
      type="button"
      onClick={open}
      aria-label="Search and run"
      aria-keyshortcuts="Meta+K Control+K /"
      className={cn(
        "flex h-9 shrink-0 items-center gap-2 rounded-md border bg-card px-2.5 text-sm text-muted-foreground shadow-xs outline-none transition-colors hover:border-primary/40 hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 md:w-72 md:px-3",
        className,
      )}
    >
      <Search size={16} className="shrink-0" aria-hidden />
      <span className="hidden flex-1 truncate text-left md:inline">Go to or run…</span>
      <kbd className="hidden font-sans text-[11px] md:inline">{modKey} K</kbd>
    </button>
  );
}
